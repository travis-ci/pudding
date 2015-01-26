package workers

import (
	"encoding/json"
	"fmt"

	"github.com/Sirupsen/logrus"
	"github.com/garyburd/redigo/redis"
	"github.com/goamz/goamz/ec2"
	"github.com/jrallison/go-workers"
	"github.com/travis-ci/pudding/lib"
	"github.com/travis-ci/pudding/lib/db"
)

func init() {
	defaultQueueFuncs["instance-terminations"] = instanceTerminationsMain
}

func instanceTerminationsMain(cfg *internalConfig, msg *workers.Msg) {
	log.WithFields(logrus.Fields{
		"jid": msg.Jid(),
	}).Debug("starting processing of termination job")

	buildPayloadJSON := []byte(msg.OriginalJson())
	buildPayload := &lib.InstanceTerminationPayload{}

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
	n   []lib.Notifier
	iid string
	cfg *internalConfig
	ec2 *ec2.EC2
}

func newInstanceTerminatorWorker(instanceID, slackChannel string, cfg *internalConfig, jid string, redisConn redis.Conn) *instanceTerminatorWorker {
	notifier := lib.NewSlackNotifier(cfg.SlackHookPath, cfg.SlackUsername, cfg.SlackIcon)

	return &instanceTerminatorWorker{
		rc:  redisConn,
		jid: jid,
		cfg: cfg,
		nc:  slackChannel,
		n:   []lib.Notifier{notifier},
		iid: instanceID,
		ec2: ec2.New(cfg.AWSAuth, cfg.AWSRegion),
	}
}

func (itw *instanceTerminatorWorker) Terminate() error {
	_, err := itw.ec2.TerminateInstances([]string{itw.iid})
	if err != nil {
		return err
	}

	err = db.RemoveInstances(itw.rc, []string{itw.iid})
	if err != nil {
		for _, notifier := range itw.n {
			notifier.Notify(itw.nc, fmt.Sprintf("Failed to terminate *%s* :scream_cat: _(%s)_", itw.iid, err))
		}
		return err
	}

	for _, notifier := range itw.n {
		notifier.Notify(itw.nc, fmt.Sprintf("Terminating *%s* :boom:", itw.iid))
	}
	return nil
}
