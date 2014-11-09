package workers

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"text/template"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/garyburd/redigo/redis"
	"github.com/gorilla/feeds"
	"github.com/jrallison/go-workers"
	"github.com/mitchellh/goamz/ec2"
	"github.com/travis-pro/worker-manager-service/lib"
	"github.com/travis-pro/worker-manager-service/lib/db"
)

func init() {
	defaultQueueFuncs["instance-builds"] = instanceBuildsMain
}

func instanceBuildsMain(cfg *internalConfig, msg *workers.Msg) {
	buildPayloadJSON := []byte(msg.OriginalJson())
	buildPayload := &lib.InstanceBuildPayload{}

	err := json.Unmarshal(buildPayloadJSON, buildPayload)
	if err != nil {
		log.WithField("err", err).Panic("failed to deserialize message")
	}

	err = newInstanceBuilderWorker(buildPayload.InstanceBuild(),
		cfg, msg.Jid(), workers.Config.Pool.Get()).Build()
	if err != nil {
		log.WithField("err", err).Panic("instance build failed")
	}
}

type instanceBuilderWorker struct {
	rc     redis.Conn
	n      []lib.Notifier
	jid    string
	cfg    *internalConfig
	ec2    *ec2.EC2
	sg     *ec2.SecurityGroup
	sgName string
	ami    *ec2.Image
	b      *lib.InstanceBuild
	i      *ec2.Instance
	t      *template.Template
}

func newInstanceBuilderWorker(b *lib.InstanceBuild, cfg *internalConfig, jid string, redisConn redis.Conn) *instanceBuilderWorker {
	notifier := lib.NewSlackNotifier(cfg.SlackTeam, cfg.SlackToken)

	ibw := &instanceBuilderWorker{
		rc:  redisConn,
		jid: jid,
		cfg: cfg,
		n:   []lib.Notifier{notifier},
		b:   b,
		ec2: ec2.New(cfg.AWSAuth, cfg.AWSRegion),
		t:   cfg.InitScriptTemplate,
	}

	ibw.sgName = fmt.Sprintf("docker-worker-%d-%p", time.Now().UTC().Unix(), ibw)
	return ibw
}

func (ibw *instanceBuilderWorker) Build() error {
	var err error

	log.WithField("jid", ibw.jid).Debug("resolving ami by id")
	ibw.ami, err = lib.ResolveAMI(ibw.ec2, ibw.b.AMI)
	if err != nil {
		log.WithFields(logrus.Fields{
			"jid":    ibw.jid,
			"ami_id": ibw.b.AMI,
			"err":    err,
		}).Error("failed to resolve ami")
		return err
	}

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

	log.WithField("jid", ibw.jid).Debug("creating instance")
	err = ibw.createInstance()
	if err != nil {
		log.WithFields(logrus.Fields{
			"err": err,
			"jid": ibw.jid,
		}).Error("failed to create instance(s)")
		return err
	}

	log.WithField("jid", ibw.jid).Debug("tagging instance")
	err = ibw.tagInstance()
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

func (ibw *instanceBuilderWorker) createSecurityGroup() error {
	newSg := ec2.SecurityGroup{
		Name:        ibw.sgName,
		Description: "custom docker worker security group",
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

	resp, err := ibw.ec2.RunInstances(&ec2.RunInstances{
		ImageId:        ibw.ami.Id,
		UserData:       userData,
		InstanceType:   ibw.b.InstanceType,
		SecurityGroups: []ec2.SecurityGroup{*ibw.sg},
	})
	if err != nil {
		return err
	}

	ibw.i = &resp.Instances[0]
	return nil
}

func (ibw *instanceBuilderWorker) tagInstance() error {
	_, err := ibw.ec2.CreateTags([]string{ibw.i.InstanceId}, []ec2.Tag{
		ec2.Tag{Key: "role", Value: "worker"},
		ec2.Tag{Key: "Name", Value: fmt.Sprintf("travis-%s-%s-%s-%s", ibw.b.Site, ibw.b.Env, ibw.b.Queue, strings.TrimPrefix(ibw.i.InstanceId, "i-"))},
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

	buf := &bytes.Buffer{}
	w, err := gzip.NewWriterLevel(buf, gzip.BestCompression)
	if err != nil {
		return nil, err
	}

	yml, err := lib.BuildTravisWorkerYML(ibw.b.Site, ibw.b.Env, ibw.cfg.TravisWorkerYML, ibw.b.Queue, ibw.b.Count)
	if err != nil {
		return nil, err
	}

	ymlString, err := yml.String()
	if err != nil {
		return nil, err
	}

	err = ibw.t.Execute(w, &initScriptContext{
		Env:              ibw.b.Env,
		Site:             ibw.b.Site,
		DockerRSA:        ibw.cfg.DockerRSA,
		SlackChannel:     ibw.b.SlackChannel,
		PapertrailSite:   yml.PapertrailSite,
		TravisWorkerYML:  ymlString,
		InstanceBuildID:  ibw.b.ID,
		InstanceBuildURL: instanceBuildURL,
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

	scriptKey := db.InitScriptRedisKey(ibw.b.ID)
	err = ibw.rc.Send("SETEX", scriptKey, 600, initScriptB64)
	if err != nil {
		ibw.rc.Send("DISCARD")
		return nil, err
	}

	authKey := db.AuthRedisKey(ibw.b.ID)
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

func (ibw *instanceBuilderWorker) notifyInstanceLaunched() {
	for _, notifier := range ibw.n {
		notifier.Notify(ibw.b.SlackChannel,
			fmt.Sprintf("Started instance `%s` for instance build *%s*", ibw.i.InstanceId, ibw.b.ID))
	}
}
