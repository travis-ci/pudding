package workers

import (
	"encoding/json"
	"fmt"
	"strings"

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
		Args: []*lib.AutoscalingGroupBuild{
			lib.NewAutoscalingGroupBuild(),
		},
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
	rc  redis.Conn
	n   []lib.Notifier
	jid string
	cfg *internalConfig
	ec2 *ec2.EC2
	as  *autoscaling.AutoScaling
	b   *lib.AutoscalingGroupBuild
}

func newAutoscalingGroupBuilderWorker(b *lib.AutoscalingGroupBuild, cfg *internalConfig, jid string, redisConn redis.Conn) *autoscalingGroupBuilderWorker {
	notifier := lib.NewSlackNotifier(cfg.SlackTeam, cfg.SlackToken)

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
	b := asgbw.b

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
			Key: "Name",
			// FIXME: build name with template as with instance builds
			// Value: name,
			Value: fmt.Sprintf("travis-%s-%s-%s-%s-asg", b.Site, b.Env, b.Queue, strings.TrimPrefix(b.InstanceID, "i-")),
		},
	}

	_, err := asgbw.as.CreateAutoScalingGroup(&autoscaling.CreateAutoScalingGroup{
		Name:            b.Name,
		InstanceId:      b.InstanceID,
		MinSize:         b.MinSize,
		MaxSize:         b.MaxSize,
		DesiredCapacity: b.DesiredCapacity,
		Tags:            tags,
	})

	if err != nil {
		log.WithFields(logrus.Fields{
			"err": err,
			"jid": asgbw.jid,
		}).Error("failed to create autoscaling group")
		return err
	}

	log.WithField("jid", asgbw.jid).Debug("all done")
	return nil
}
