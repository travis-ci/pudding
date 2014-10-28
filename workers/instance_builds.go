package workers

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/garyburd/redigo/redis"
	"github.com/gorilla/feeds"
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
		cfg, msg.Jid(), workers.Config.Pool.Get()).Build()
	if err != nil {
		log.WithField("err", err).Panic("instance build failed")
	}
}

type instanceBuilderWorker struct {
	rc     redis.Conn
	jid    string
	cfg    *config
	ec2    *ec2.EC2
	sg     *ec2.SecurityGroup
	sgName string
	ami    *ec2.Image
	b      *common.InstanceBuild
	i      []*ec2.Instance
}

func newInstanceBuilderWorker(b *common.InstanceBuild, cfg *config, jid string, redisConn redis.Conn) *instanceBuilderWorker {
	return &instanceBuilderWorker{
		rc:     redisConn,
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

	log.WithField("jid", ibw.jid).Info("tagging instances")
	err = ibw.tagInstances()
	if err != nil {
		log.WithFields(logrus.Fields{
			"err": err,
			"jid": ibw.jid,
		}).Error("failed to tag instance(s)")
		return err
	}

	ibw.notifyInstancesLaunched()

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

	userData, err := ibw.buildUserData()
	if err != nil {
		return err
	}

	resp, err := ibw.ec2.RunInstances(&ec2.RunInstances{
		ImageId:        ibw.ami.Id,
		MinCount:       ibw.b.Count,
		MaxCount:       ibw.b.Count,
		UserData:       userData,
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

func (ibw *instanceBuilderWorker) buildUserData() ([]byte, error) {
	webURL, err := url.Parse(ibw.cfg.WebHost)
	if err != nil {
		return nil, err
	}

	tmpAuth := feeds.NewUUID().String()
	webURL.User = url.UserPassword("x", tmpAuth)

	webURL.Path = fmt.Sprintf("/init-scripts/%s", ibw.b.ID)
	initScriptURL := webURL.String()

	webURL.Path = fmt.Sprintf("/instance-builds/%s", ibw.b.ID)
	instanceBuildURL := webURL.String()

	webURL.Path = "/instances/$INSTANCE_ID/links/metadata"
	instanceMetadataURL := webURL.String()

	buf := &bytes.Buffer{}
	w, err := gzip.NewWriterLevel(buf, gzip.BestCompression)
	if err != nil {
		return nil, err
	}

	err = initScript.Execute(w, &initScriptContext{
		DockerRSA:           ibw.cfg.DockerRSA,
		PapertrailSite:      ibw.cfg.PapertrailSite,
		TravisWorkerYML:     ibw.cfg.TravisWorkerYML,
		InstanceBuildID:     ibw.b.ID,
		InstanceBuildURL:    instanceBuildURL,
		InstanceMetadataURL: instanceMetadataURL,
	})
	if err != nil {
		return nil, err
	}

	err = w.Close()
	if err != nil {
		return nil, err
	}

	initScriptB64 := base64.StdEncoding.EncodeToString(buf.Bytes())

	err = ibw.rc.Send("MULTI")
	if err != nil {
		return nil, err
	}

	scriptKey := common.InitScriptRedisKey(ibw.b.ID)
	err = ibw.rc.Send("SETEX", scriptKey, 600, initScriptB64)
	if err != nil {
		ibw.rc.Send("DISCARD")
		return nil, err
	}

	authKey := common.AuthRedisKey(ibw.b.ID)
	err = ibw.rc.Send("SETEX", authKey, 600, tmpAuth)
	if err != nil {
		ibw.rc.Send("DISCARD")
		return nil, err
	}

	_, err = ibw.rc.Do("EXEC")
	if err != nil {
		return nil, err
	}

	return []byte(fmt.Sprintf("#include %s\n", initScriptURL)), nil
}

func (ibw *instanceBuilderWorker) notifyInstancesLaunched() {
	// TODO: notify instance launched in Slack or some such
	log.WithFields(logrus.Fields{
		"jid":          ibw.jid,
		"instance_ids": ibw.instanceIDs(),
	}).Info("launched instances")
}

func (ibw *instanceBuilderWorker) instanceIDs() []string {
	out := []string{}
	for _, inst := range ibw.i {
		out = append(out, inst.InstanceId)
	}
	return out
}
