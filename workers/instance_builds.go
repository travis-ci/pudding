package workers

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"text/template"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/garyburd/redigo/redis"
	"github.com/goamz/goamz/ec2"
	"github.com/gorilla/feeds"
	"github.com/jrallison/go-workers"
	"github.com/travis-ci/pudding"
)

func init() {
	defaultQueueFuncs["instance-builds"] = instanceBuildsMain
}

func instanceBuildsMain(cfg *internalConfig, msg *workers.Msg) {
	buildPayloadJSON := []byte(msg.OriginalJson())
	buildPayload := &pudding.InstanceBuildPayload{
		Args: []*pudding.InstanceBuild{
			pudding.NewInstanceBuild(),
		},
	}

	err := json.Unmarshal(buildPayloadJSON, buildPayload)
	if err != nil {
		log.WithField("err", err).Panic("failed to deserialize message")
	}

	b := buildPayload.InstanceBuild()
	b.Hydrate()

	ibw, err := newInstanceBuilderWorker(b, cfg, msg.Jid(), workers.Config.Pool.Get())
	if err != nil {
		log.WithField("err", err).Panic("failed to make an instance build worker")
	}

	if b.BootInstance == nil {
		bootInstance := new(bool)
		*bootInstance = true
		b.BootInstance = bootInstance
	}

	if *b.BootInstance {
		ibw.Build()
	} else {
		ibw.CreateUserData()
	}

	if err != nil {
		log.WithField("err", err).Panic("instance build failed")
	}
}

type instanceBuilderWorker struct {
	rc     redis.Conn
	n      []pudding.Notifier
	jid    string
	cfg    *internalConfig
	ec2    *ec2.EC2
	sg     *ec2.SecurityGroup
	sgName string
	ami    *ec2.Image
	b      *pudding.InstanceBuild
	i      *ec2.Instance
	t      *template.Template
}

func newInstanceBuilderWorker(b *pudding.InstanceBuild, cfg *internalConfig, jid string, redisConn redis.Conn) (*instanceBuilderWorker, error) {
	var err error
	notifier := pudding.NewSlackNotifier(cfg.SlackHookPath, cfg.SlackUsername, cfg.SlackIcon)

	t := template.New("init-script")
	t.Funcs(template.FuncMap{
		"env_for":    pudding.MakeInstanceBuildEnvForFunc(b),
		"env":        os.Getenv,
		"uncompress": pudding.MakeTemplateUncompressFunc(log),
	})

	t, err = t.Parse(cfg.InitScriptTemplateString)
	if err != nil {
		return nil, err
	}

	ibw := &instanceBuilderWorker{
		rc:  redisConn,
		jid: jid,
		cfg: cfg,
		n:   []pudding.Notifier{notifier},
		b:   b,
		ec2: ec2.New(cfg.AWSAuth, cfg.AWSRegion),
		t:   t,
	}

	ibw.sgName = fmt.Sprintf("pudding-%d-%p", time.Now().UTC().Unix(), ibw)
	return ibw, nil
}

func (ibw *instanceBuilderWorker) Build() error {
	var err error

	f := ec2.NewFilter()
	if ibw.b.Role != "" {
		f.Add("tag:role", ibw.b.Role)
	}

	log.WithFields(logrus.Fields{
		"jid":    ibw.jid,
		"filter": f,
	}).Debug("resolving ami")

	ibw.ami, err = pudding.ResolveAMI(ibw.ec2, ibw.b.AMI, f)
	if err != nil {
		log.WithFields(logrus.Fields{
			"jid":    ibw.jid,
			"ami_id": ibw.b.AMI,
			"err":    err,
		}).Error("failed to resolve ami")
		return err
	}

	if ibw.b.SecurityGroupID != "" {
		ibw.sg = &ec2.SecurityGroup{Id: ibw.b.SecurityGroupID}
	} else {
		log.WithField("jid", ibw.jid).Debug("creating security group")
		err = ibw.createSecurityGroup()
		if err != nil {
			log.WithFields(logrus.Fields{
				"jid": ibw.jid,
				"security_group_name": ibw.sgName,
				"err": err,
			}).Error("failed to create security group")
			return err
		}
	}

	log.WithField("jid", ibw.jid).Debug("creating instance")
	err = ibw.createInstance()
	if err != nil {
		log.WithFields(logrus.Fields{
			"err":         err,
			"err_details": fmt.Sprintf("%#v", err),
			"jid":         ibw.jid,
		}).Error("failed to create instance(s)")
		return err
	}

	ibw.b.InstanceID = ibw.i.InstanceId

	for i := ibw.cfg.InstanceTagRetries; i > 0; i-- {
		log.WithField("jid", ibw.jid).Debug("tagging instance")
		err = ibw.tagInstance()
		if err == nil {
			break
		}
		time.Sleep(3 * time.Second)
	}

	if err != nil {
		log.WithFields(logrus.Fields{
			"err": err,
			"jid": ibw.jid,
		}).Error("failed to tag instance(s)")
		return err
	}

	ibw.notifyInstanceLaunched()

	log.WithField("jid", ibw.jid).Debug("all done")
	return nil
}

func (ibw *instanceBuilderWorker) CreateUserData() ([]byte, error) {
	log.WithFields(logrus.Fields{
		"jid":           ibw.jid,
		"instance_type": ibw.b.InstanceType,
	}).Info("creating user data")

	return ibw.buildUserData()
}

func (ibw *instanceBuilderWorker) createSecurityGroup() error {
	newSg := ec2.SecurityGroup{
		Name:        ibw.sgName,
		Description: "custom security group",
	}

	log.WithFields(logrus.Fields{
		"jid": ibw.jid,
		"security_group_name": ibw.sgName,
	}).Debug("creating security group")

	resp, err := ibw.ec2.CreateSecurityGroup(newSg)
	if err != nil {
		log.WithFields(logrus.Fields{
			"err": err,
			"jid": ibw.jid,
		}).Error("failed to create security group")
		return err
	}

	ibw.sg = &resp.SecurityGroup

	log.WithFields(logrus.Fields{
		"jid": ibw.jid,
		"security_group_name": ibw.sgName,
	}).Debug("authorizing port 22 on security group")

	_, err = ibw.ec2.AuthorizeSecurityGroup(*ibw.sg, []ec2.IPPerm{
		ec2.IPPerm{
			Protocol:  "tcp",
			FromPort:  22,
			ToPort:    22,
			SourceIPs: []string{"0.0.0.0/0"},
		},
	})
	if err != nil {
		log.WithFields(logrus.Fields{
			"err": err,
			"jid": ibw.jid,
			"security_group_name": ibw.sgName,
		}).Error("failed to authorize port 22")
		return err
	}

	return nil
}

func (ibw *instanceBuilderWorker) createInstance() error {
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

	resp, err := ibw.ec2.RunInstances(&ec2.RunInstancesOptions{
		ImageId:        ibw.ami.Id,
		UserData:       userData,
		InstanceType:   ibw.b.InstanceType,
		SecurityGroups: []ec2.SecurityGroup{*ibw.sg},
		SubnetId:       ibw.b.SubnetID,
	})
	if err != nil {
		return err
	}

	ibw.i = &resp.Instances[0]
	return nil
}

func (ibw *instanceBuilderWorker) tagInstance() error {
	nameTmpl, err := template.New(fmt.Sprintf("name-template-%s", ibw.jid)).Parse(ibw.b.NameTemplate)
	if err != nil {
		return err
	}

	var nameBuf bytes.Buffer
	err = nameTmpl.Execute(&nameBuf, ibw.b)
	if err != nil {
		return err
	}

	tags := []ec2.Tag{
		ec2.Tag{Key: "Name", Value: nameBuf.String()},
		ec2.Tag{Key: "role", Value: ibw.b.Role},
		ec2.Tag{Key: "site", Value: ibw.b.Site},
		ec2.Tag{Key: "env", Value: ibw.b.Env},
		ec2.Tag{Key: "queue", Value: ibw.b.Queue},
	}

	log.WithFields(logrus.Fields{
		"jid":  ibw.jid,
		"tags": tags,
	}).Debug("tagging instance")

	_, err = ibw.ec2.CreateTags([]string{ibw.i.InstanceId}, tags)

	return err
}

func (ibw *instanceBuilderWorker) buildUserData() ([]byte, error) {
	webURL, err := url.Parse(ibw.cfg.WebHost)
	if err != nil {
		return nil, err
	}

	instAuth := feeds.NewUUID().String()
	webURL.User = url.UserPassword("x", instAuth)

	webURL.Path = fmt.Sprintf("/instance-launches/%s", ibw.b.ID)
	instanceLaunchURL := webURL.String()

	webURL.Path = fmt.Sprintf("/instance-terminations/%s", ibw.b.ID)
	instanceTerminateURL := webURL.String()

	webURL.Path = fmt.Sprintf("/instance-builds/%s", ibw.b.ID)
	instanceBuildURL := webURL.String()

	webURL.Path = fmt.Sprintf("/init-scripts/%s", ibw.b.ID)
	initScriptURL := webURL.String()

	webURL.Path = fmt.Sprintf("/instance-heartbeats/%s", ibw.b.ID)
	instanceHeartbeatURL := webURL.String()

	buf := &bytes.Buffer{}
	gzw, err := gzip.NewWriterLevel(buf, gzip.BestCompression)
	if err != nil {
		return nil, err
	}

	tw := &bytes.Buffer{}
	w := io.MultiWriter(tw, gzw)

	yml, err := pudding.BuildInstanceSpecificYML(ibw.b.Site, ibw.b.Env, ibw.cfg.InstanceYML, ibw.b.Queue, ibw.b.Count)
	if err != nil {
		return nil, err
	}

	ymlString, err := yml.String()
	if err != nil {
		return nil, err
	}

	err = ibw.t.Execute(w, &initScriptContext{
		Env:                  ibw.b.Env,
		Site:                 ibw.b.Site,
		Queue:                ibw.b.Queue,
		Role:                 ibw.b.Role,
		AMI:                  ibw.b.AMI,
		Count:                ibw.b.Count,
		SlackChannel:         ibw.b.SlackChannel,
		InstanceType:         ibw.b.InstanceType,
		InstanceBuildID:      ibw.b.ID,
		InstanceBuildURL:     instanceBuildURL,
		InstanceLaunchURL:    instanceLaunchURL,
		InstanceTerminateURL: instanceTerminateURL,
		InstanceHeartbeatURL: instanceHeartbeatURL,

		// TODO: extract InstanceRSA key via `env` func
		InstanceRSA: ibw.cfg.InstanceRSA,
		// TODO: extract PapertrailSite key via `instance_env` func
		PapertrailSite: yml.PapertrailSite,
		// TODO: extract InstanceYML key via an `instance_env` func
		InstanceYML: ymlString,
	})
	if err != nil {
		return nil, err
	}

	log.WithFields(logrus.Fields{
		"jid":    ibw.jid,
		"script": tw.String(),
	}).Debug("rendered init script")

	err = gzw.Close()
	if err != nil {
		return nil, err
	}

	initScriptB64 := base64.StdEncoding.EncodeToString(buf.Bytes())

	err = ibw.rc.Send("MULTI")
	if err != nil {
		return nil, err
	}

	err = ibw.rc.Send("HSET", fmt.Sprintf("%s:init-scripts", pudding.RedisNamespace), ibw.b.ID, initScriptB64)
	if err != nil {
		ibw.rc.Send("DISCARD")
		return nil, err
	}

	err = ibw.rc.Send("HSET", fmt.Sprintf("%s:auths", pudding.RedisNamespace), ibw.b.ID, instAuth)
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

func (ibw *instanceBuilderWorker) notifyInstanceLaunched() {
	for _, notifier := range ibw.n {
		notifier.Notify(ibw.b.SlackChannel,
			fmt.Sprintf("Started instance `%s` for instance build *%s* %s",
				ibw.i.InstanceId, ibw.b.ID, pudding.NotificationInstanceBuildSummary(ibw.b)))
	}
}
