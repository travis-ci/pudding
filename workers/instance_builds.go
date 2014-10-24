package workers

import (
	"encoding/json"
	"fmt"
	"time"

	"code.google.com/p/go.crypto/ssh"

	"github.com/Sirupsen/logrus"
	"github.com/jrallison/go-workers"
	"github.com/mitchellh/goamz/ec2"
	"github.com/travis-pro/worker-manager-service/common"
)

func init() {
	defaultQueueFuncs["instance-builds"] = instanceBuildsMain
}

func instanceBuildsMain(cfg *config, msg *workers.Msg) {
	buildPayloadJSON := []byte(msg.OriginalJson())
	buildPayload := &common.InstanceBuildPayload{}

	err := json.Unmarshal(buildPayloadJSON, buildPayload)
	if err != nil {
		log.WithField("err", err).Error("failed to deserialize message")
	}

	err = newInstanceBuilderWorker(buildPayload.InstanceBuild(),
		cfg, msg.Jid()).Build()
	if err != nil {
		log.WithField("err", err).Panic("instance build failed")
	}
}

type instanceBuilderWorker struct {
	jid    string
	cfg    *config
	ec2    *ec2.EC2
	sg     *ec2.SecurityGroup
	sgName string
	ami    *ec2.Image
	b      *common.InstanceBuild
	i      []*ec2.Instance
}

func newInstanceBuilderWorker(b *common.InstanceBuild, cfg *config, jid string) *instanceBuilderWorker {
	return &instanceBuilderWorker{
		jid:    jid,
		cfg:    cfg,
		b:      b,
		i:      []*ec2.Instance{},
		sgName: fmt.Sprintf("docker-worker-%d", time.Now().UTC().Unix()),
		ec2:    ec2.New(cfg.AWSAuth, cfg.AWSRegion),
	}
}

func (ibw *instanceBuilderWorker) Build() error {
	var err error

	log.WithField("jid", ibw.jid).Info("resolving ami by id")
	ibw.ami, err = common.ResolveAMI(ibw.ec2, ibw.b.AMI)
	if err != nil {
		log.WithFields(logrus.Fields{
			"jid":    ibw.jid,
			"ami_id": ibw.b.AMI,
			"err":    err,
		}).Error("failed to resolve ami")
		return err
	}

	log.WithField("jid", ibw.jid).Info("creating security group")
	err = ibw.createSecurityGroup()
	if err != nil {
		log.WithFields(logrus.Fields{
			"jid": ibw.jid,
			"security_group_name": ibw.sgName,
			"err": err,
		}).Error("failed to create security group")
		return err
	}

	log.WithField("jid", ibw.jid).Info("creating instances")
	err = ibw.createInstances()
	if err != nil {
		log.WithFields(logrus.Fields{
			"err": err,
			"jid": ibw.jid,
		}).Error("failed to create instance(s)")
		return err
	}

	log.WithField("jid", ibw.jid).Info("tagging instances instances")
	err = ibw.tagInstances()
	if err != nil {
		log.WithFields(logrus.Fields{
			"err": err,
			"jid": ibw.jid,
		}).Error("failed to tag instance(s)")
		return err
	}

	log.WithField("jid", ibw.jid).Info("waiting for instances")
	err = ibw.waitForInstances()
	if err != nil {
		log.WithFields(logrus.Fields{
			"err": err,
			"jid": ibw.jid,
		}).Error("failed to wait for instance(s)")
		return err
	}

	ibw.notifyInstancesLaunched()

	log.WithField("jid", ibw.jid).Info("setting up instances")
	err = ibw.setupInstances()
	if err != nil {
		log.WithFields(logrus.Fields{
			"err": err,
			"jid": ibw.jid,
		}).Error("failed to set up instance(s)")
		return err
	}

	log.WithField("jid", ibw.jid).Info("all done")
	return nil
}

func (ibw *instanceBuilderWorker) createSecurityGroup() error {
	newSg := ec2.SecurityGroup{
		Name:        ibw.sgName,
		Description: "custom docker worker security group",
	}
	resp, err := ibw.ec2.CreateSecurityGroup(newSg)
	if err != nil {
		log.WithFields(logrus.Fields{
			"err": err,
			"jid": ibw.jid,
		}).Error("failed to create security group")
		return err
	}

	ibw.sg = &resp.SecurityGroup
	return nil
}

func (ibw *instanceBuilderWorker) createInstances() error {
	log.WithFields(logrus.Fields{
		"jid":           ibw.jid,
		"instance_type": ibw.b.InstanceType,
		"ami.id":        ibw.ami.Id,
		"ami.name":      ibw.ami.Name,
		"count":         ibw.b.Count,
	}).Info("booting instance")

	resp, err := ibw.ec2.RunInstances(&ec2.RunInstances{
		ImageId:        ibw.ami.Id,
		InstanceType:   ibw.b.InstanceType,
		SecurityGroups: []ec2.SecurityGroup{*ibw.sg},
	})
	if err != nil {
		return err
	}

	for _, inst := range resp.Instances {
		ibw.i = append(ibw.i, &inst)
	}

	return nil
}

func (ibw *instanceBuilderWorker) tagInstances() error {
	_, err := ibw.ec2.CreateTags(ibw.instanceIDs(), []ec2.Tag{
		ec2.Tag{Key: "role", Value: "worker"},
		ec2.Tag{Key: "Name", Value: fmt.Sprintf("travis-%s-%s-%s", ibw.b.Site, ibw.b.Env, ibw.b.Queue)},
		ec2.Tag{Key: "site", Value: ibw.b.Site},
		ec2.Tag{Key: "env", Value: ibw.b.Env},
		ec2.Tag{Key: "queue", Value: ibw.b.Queue},
	})

	return err
}

func (ibw *instanceBuilderWorker) waitForInstances() error {
	for {
		resp, err := ibw.ec2.Instances(ibw.instanceIDs(), ec2.NewFilter())

		if err != nil {
			log.WithFields(logrus.Fields{
				"err": err,
				"jid": ibw.jid,
			}).Warn("failed to get status while waiting for instances")
			time.Sleep(5 * time.Second)
			continue
		}

		if resp == nil || len(resp.Reservations) == 0 || len(resp.Reservations[0].Instances) == 0 {
			log.WithFields(logrus.Fields{
				"err": err,
				"jid": ibw.jid,
			}).Warn("still waiting for instance")
			time.Sleep(5 * time.Second)
			continue
		}

		statuses := map[string]int{}

		for _, res := range resp.Reservations {
			for _, inst := range res.Instances {
				statuses[inst.State.Name] = 1
			}
		}

		if _, ok := statuses["pending"]; !ok {
			return nil
		}

		log.WithFields(logrus.Fields{
			"err": err,
			"jid": ibw.jid,
		}).Warn("still waiting for instance")
		time.Sleep(5 * time.Second)
	}
}

func (ibw *instanceBuilderWorker) notifyInstancesLaunched() {
	// TODO: notify instance launched in Slack or some such
	log.WithFields(logrus.Fields{
		"jid":          ibw.jid,
		"instance_ids": ibw.instanceIDs(),
	}).Info("launched instances")
}

func (ibw *instanceBuilderWorker) setupInstances() error {
	// TODO: setup instance
	errors := []error{}

	for _, inst := range ibw.i {
		ipv4, err := common.GetInstanceIPv4(ibw.ec2, inst.InstanceId)
		if err != nil {
			errors = append(errors, err)
			continue
		}

		setupCfg := &instanceSetupConfig{
			JID:             ibw.jid,
			InstanceID:      inst.InstanceId,
			InstanceIPv4:    ipv4,
			SSHUser:         "moustache", // TODO: pass in ssh user?
			SetupRSA:        ibw.cfg.SetupRSA,
			DockerRSA:       ibw.cfg.DockerRSA,
			WorkerConfigURL: "http://example.com", // TODO: implement worker config fetching, maybe etcd/consul?
			PapertrailSite:  "TODO",               // TODO: papertrail site is wat?
			MaxRetries:      10,                   // TODO: sane max retries?
		}

		isu := newInstanceSetterUpper(setupCfg)
		err = isu.SetupInstance()
		if err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return &common.MultiError{Errors: errors}
	}

	return nil
}

func (ibw *instanceBuilderWorker) instanceIDs() []string {
	out := []string{}
	for _, inst := range ibw.i {
		out = append(out, inst.InstanceId)
	}
	return out
}

type instanceSetupConfig struct {
	JID             string
	InstanceID      string
	InstanceIPv4    string
	SSHUser         string
	SetupRSA        string
	DockerRSA       string
	WorkerConfigURL string
	PapertrailSite  string
	MaxRetries      int
}

type instanceSetterUpper struct {
	c   *instanceSetupConfig
	ssh *ssh.Client
}

func newInstanceSetterUpper(c *instanceSetupConfig) *instanceSetterUpper {
	if c.MaxRetries == 0 {
		c.MaxRetries = 10
	}
	return &instanceSetterUpper{c: c}
}

func (isu *instanceSetterUpper) SetupInstance() error {
	nRetries := 0

	for {
		err := isu.setupInstanceAttempt()
		if err != nil {
			if nRetries > isu.c.MaxRetries {
				return err
			}
			nRetries++
			time.Sleep(1 * time.Second)
			continue
		}
		return nil
	}
}

func (isu *instanceSetterUpper) setupInstanceAttempt() error {
	log.WithFields(logrus.Fields{
		"jid":         isu.c.JID,
		"instance_id": isu.c.InstanceID,
	}).Info("getting ssh connection")

	err := isu.getSSHConnection()
	if err != nil {
		log.WithFields(logrus.Fields{
			"err":         err,
			"jid":         isu.c.JID,
			"instance_id": isu.c.InstanceID,
		}).Error("failed to get ssh connection")
		return err
	}

	log.WithFields(logrus.Fields{
		"jid":         isu.c.JID,
		"instance_id": isu.c.InstanceID,
	}).Info("uploading docker rsa")

	err = isu.uploadDockerRSA()
	if err != nil {
		log.WithFields(logrus.Fields{
			"err":         err,
			"jid":         isu.c.JID,
			"instance_id": isu.c.InstanceID,
		}).Error("failed to upload docker rsa")
		return err
	}

	log.WithFields(logrus.Fields{
		"jid":         isu.c.JID,
		"instance_id": isu.c.InstanceID,
	}).Info("uploading worker config")

	err = isu.uploadWorkerConfig()
	if err != nil {
		log.WithFields(logrus.Fields{
			"err":         err,
			"jid":         isu.c.JID,
			"instance_id": isu.c.InstanceID,
		}).Error("failed to upload worker config")
		return err
	}

	log.WithFields(logrus.Fields{
		"jid":         isu.c.JID,
		"instance_id": isu.c.InstanceID,
	}).Info("setting up papertrail")

	err = isu.setupPapertrail()
	if err != nil {
		log.WithFields(logrus.Fields{
			"err":         err,
			"jid":         isu.c.JID,
			"instance_id": isu.c.InstanceID,
		}).Error("failed to setup papertrail")
		return err
	}

	return nil
}

func (isu *instanceSetterUpper) getSSHConnection() error {
	signer, err := ssh.ParsePrivateKey([]byte(isu.c.SetupRSA))
	if err != nil {
		return err
	}

	sshConfig := &ssh.ClientConfig{
		User: isu.c.SSHUser,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
	}

	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:22", isu.c.InstanceIPv4), sshConfig)
	if err != nil {
		return err
	}

	isu.ssh = client
	return nil
}

func (isu *instanceSetterUpper) uploadDockerRSA() error {
	// TODO: upload docker RSA
	return nil
}

func (isu *instanceSetterUpper) uploadWorkerConfig() error {
	// TODO: upload worker config
	return nil
}

func (isu *instanceSetterUpper) setupPapertrail() error {
	// TODO: setup papertrail
	return nil
}
