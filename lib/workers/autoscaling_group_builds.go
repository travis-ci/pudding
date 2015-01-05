package workers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"

	"github.com/Sirupsen/logrus"
	"github.com/garyburd/redigo/redis"
	"github.com/jrallison/go-workers"
	"github.com/mitchellh/goamz/autoscaling"
	"github.com/mitchellh/goamz/ec2"
	"github.com/travis-ci/pudding/lib"
)

func init() {
	defaultQueueFuncs["autoscaling-group-builds"] = autoscalingGroupBuildsMain
}

func autoscalingGroupBuildsMain(cfg *internalConfig, msg *workers.Msg) {
	buildPayloadJSON := []byte(msg.OriginalJson())
	buildPayload := &lib.AutoscalingGroupBuildPayload{
		Args: []*lib.AutoscalingGroupBuild{},
	}

	err := json.Unmarshal(buildPayloadJSON, buildPayload)
	if err != nil {
		log.WithField("err", err).Panic("failed to deserialize message")
	}

	b := buildPayload.AutoscalingGroupBuild()
	b.Hydrate()

	err = newAutoscalingGroupBuilderWorker(b, cfg, msg.Jid(), workers.Config.Pool.Get()).Build()
	if err != nil {
		log.WithField("err", err).Panic("autoscaling group build failed")
	}
}

type autoscalingGroupBuilderWorker struct {
	rc     redis.Conn
	n      []lib.Notifier
	jid    string
	cfg    *internalConfig
	ec2    *ec2.EC2
	as     *autoscaling.AutoScaling
	b      *lib.AutoscalingGroupBuild
	sopARN string
	sipARN string
}

func newAutoscalingGroupBuilderWorker(b *lib.AutoscalingGroupBuild, cfg *internalConfig, jid string, redisConn redis.Conn) *autoscalingGroupBuilderWorker {
	notifier := lib.NewSlackNotifier(cfg.SlackHookPath, cfg.SlackUsername, cfg.SlackIcon)

	return &autoscalingGroupBuilderWorker{
		rc:  redisConn,
		jid: jid,
		cfg: cfg,
		n:   []lib.Notifier{notifier},
		b:   b,
		ec2: ec2.New(cfg.AWSAuth, cfg.AWSRegion),
		as:  autoscaling.New(cfg.AWSAuth, cfg.AWSRegion),
	}
}

func (asgbw *autoscalingGroupBuilderWorker) Build() error {
	asg, err := asgbw.createAutoscalingGroup()
	if err != nil {
		log.WithFields(logrus.Fields{
			"err": err,
			"jid": asgbw.jid,
		}).Error("failed to create autoscaling group")
		return err
	}

	sopARN, err := asgbw.createScaleOutPolicy()
	if err != nil {
		log.WithFields(logrus.Fields{
			"err":  err,
			"name": asg.Name,
			"jid":  asgbw.jid,
		}).Error("failed to create scale out policy")
		return err
	}

	asgbw.sopARN = sopARN

	sipARN, err := asgbw.createScaleInPolicy()
	if err != nil {
		log.WithFields(logrus.Fields{
			"err":  err,
			"name": asg.Name,
			"jid":  asgbw.jid,
		}).Error("failed to create scale in policy")
		return err
	}

	asgbw.sipARN = sipARN

	err = asgbw.createScaleOutMetricAlarm()
	if err != nil {
		log.WithFields(logrus.Fields{
			"err":  err,
			"name": asg.Name,
			"jid":  asgbw.jid,
		}).Error("failed to create scale out metric alarm")
		return err
	}

	err = asgbw.createScaleInMetricAlarm()
	if err != nil {
		log.WithFields(logrus.Fields{
			"err":  err,
			"name": asg.Name,
			"jid":  asgbw.jid,
		}).Error("failed to create scale in metric alarm")
		return err
	}

	err = asgbw.createLaunchingLifecycleHook()
	if err != nil {
		log.WithFields(logrus.Fields{
			"err":  err,
			"name": asg.Name,
			"jid":  asgbw.jid,
		}).Error("failed to create launching lifecycle hook")
		return err
	}

	err = asgbw.createTerminatingLifecycleHook()
	if err != nil {
		log.WithFields(logrus.Fields{
			"err":  err,
			"name": asg.Name,
			"jid":  asgbw.jid,
		}).Error("failed to create terminating lifecycle hook")
		return err
	}

	log.WithField("jid", asgbw.jid).Debug("all done")
	return nil
}

func (asgbw *autoscalingGroupBuilderWorker) createAutoscalingGroup() (*autoscaling.CreateAutoScalingGroup, error) {
	b := asgbw.b

	nameTmpl, err := template.New(fmt.Sprintf("name-template-%s", asgbw.jid)).Parse(b.NameTemplate)
	if err != nil {
		return nil, err
	}

	var nameBuf bytes.Buffer
	err = nameTmpl.Execute(&nameBuf, b)
	if err != nil {
		return nil, err
	}

	tags := []autoscaling.Tag{
		autoscaling.Tag{
			Key:   "role",
			Value: b.Role,
		},
		autoscaling.Tag{
			Key:   "queue",
			Value: b.Queue,
		},
		autoscaling.Tag{
			Key:   "site",
			Value: b.Site,
		},
		autoscaling.Tag{
			Key:   "env",
			Value: b.Env,
		},
		autoscaling.Tag{
			Key:   "Name",
			Value: nameBuf.String(),
		},
	}

	asg := &autoscaling.CreateAutoScalingGroup{
		Name:               b.Name,
		InstanceId:         b.InstanceID,
		MinSize:            b.MinSize,
		MaxSize:            b.MaxSize,
		DesiredCapacity:    b.DesiredCapacity,
		Tags:               tags,
		SetMinSize:         true,
		SetMaxSize:         true,
		SetDesiredCapacity: true,
	}

	log.WithFields(logrus.Fields{
		"jid": asgbw.jid,
		"asg": fmt.Sprintf("%#v", asg),
	}).Debug("creating autoscaling group")

	_, err = asgbw.as.CreateAutoScalingGroup(asg)
	return asg, err
}

func (asgbw *autoscalingGroupBuilderWorker) createScaleOutPolicy() (string, error) {
	return "", nil
}

func (asgbw *autoscalingGroupBuilderWorker) createScaleInPolicy() (string, error) {
	return "", nil
}

func (asgbw *autoscalingGroupBuilderWorker) createScaleOutMetricAlarm() error {
	return nil
}

func (asgbw *autoscalingGroupBuilderWorker) createScaleInMetricAlarm() error {
	return nil
}

func (asgbw *autoscalingGroupBuilderWorker) createLaunchingLifecycleHook() error {
	return nil
}

func (asgbw *autoscalingGroupBuilderWorker) createTerminatingLifecycleHook() error {
	return nil
}
