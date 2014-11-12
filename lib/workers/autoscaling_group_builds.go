package workers

import (
	"encoding/json"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/jrallison/go-workers"
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

	err = newAutoscalingGroupBuilderWorker(buildPayload.AutoscalingGroupBuild(),
		cfg, msg.Jid(), workers.Config.Pool.Get()).Build()
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
	}
}

func (asgbw *autoscalingGroupBuilderWorker) Build() error {
	time.Sleep(3 * time.Second)
	log.WithField("jid", asgbw.jid).Debug("all done")
	return nil
}
