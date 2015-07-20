package workers

import (
	"encoding/json"
	"fmt"

	"github.com/Sirupsen/logrus"
	"github.com/awslabs/aws-sdk-go/aws"
	"github.com/awslabs/aws-sdk-go/service/ec2"
	"github.com/garyburd/redigo/redis"
	"github.com/jrallison/go-workers"
	"github.com/travis-ci/pudding"
	"github.com/travis-ci/pudding/db"
)

func init() {
	defaultQueueFuncs["instance-terminations"] = instanceTerminationsMain
}

func instanceTerminationsMain(cfg *internalConfig, msg *workers.Msg) {
	log.WithFields(logrus.Fields{
		"jid": msg.Jid(),
	}).Debug("starting processing of termination job")

	buildPayloadJSON := []byte(msg.OriginalJson())
	buildPayload := &pudding.InstanceTerminationPayload{}

	err := json.Unmarshal(buildPayloadJSON, buildPayload)
	if err != nil {
		log.WithField("err", err).Panic("failed to deserialize message")
	}

	err = newInstanceTerminatorWorker(buildPayload.InstanceID, buildPayload.SlackChannel,
		cfg, msg.Jid(), workers.Config.Pool.Get()).Terminate()
	if err != nil {
		log.WithField("err", err).Panic("instance termination failed")
	}
}

type instanceTerminatorWorker struct {
	rc  redis.Conn
	jid string
	nc  string
	n   []pudding.Notifier
	iid string
	cfg *internalConfig
	ec2 *ec2.EC2
}

func newInstanceTerminatorWorker(instanceID, slackChannel string, cfg *internalConfig, jid string, redisConn redis.Conn) *instanceTerminatorWorker {
	notifier := pudding.NewSlackNotifier(cfg.SlackHookPath, cfg.SlackUsername, cfg.SlackIcon)

	return &instanceTerminatorWorker{
		rc:  redisConn,
		jid: jid,
		cfg: cfg,
		nc:  slackChannel,
		n:   []pudding.Notifier{notifier},
		iid: instanceID,
		ec2: ec2.New(cfg.AWSConfig),
	}
}

func (itw *instanceTerminatorWorker) Terminate() error {
	_, err := itw.ec2.TerminateInstances(&ec2.TerminateInstancesInput{
		InstanceIDs: []*string{
			aws.String(itw.iid),
		},
	})

	if err != nil {
		return err
	}

	instances, _ := db.FetchInstances(itw.rc, map[string]string{"instance_id": itw.iid})

	err = db.RemoveInstances(itw.rc, []string{itw.iid})
	if err != nil && instances != nil && len(instances) > 0 {
		inst := instances[0]
		for _, notifier := range itw.n {
			notifier.Notify(itw.nc,
				fmt.Sprintf("Failed to terminate *%s* :scream_cat: _(%s)_ %s",
					itw.iid, err, pudding.NotificationInstanceSummary(inst)))
		}
		return err
	}

	if instances != nil && len(instances) > 0 {
		inst := instances[0]
		for _, notifier := range itw.n {
			notifier.Notify(itw.nc, fmt.Sprintf("Terminating *%s* :boom: %s",
				itw.iid, pudding.NotificationInstanceSummary(inst)))
		}
	}
	return nil
}
