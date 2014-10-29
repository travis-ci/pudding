package workers

import (
	"encoding/json"
	"fmt"

	"github.com/Sirupsen/logrus"
	"github.com/garyburd/redigo/redis"
	"github.com/jrallison/go-workers"
	"github.com/mitchellh/goamz/ec2"
	"github.com/travis-pro/worker-manager-service/common"
)

func init() {
	defaultQueueFuncs["instance-terminations"] = instanceTerminationsMain
}

func instanceTerminationsMain(cfg *config, msg *workers.Msg) {
	log.WithFields(logrus.Fields{
		"jid": msg.Jid(),
	}).Debug("starting processing of termination job")

	buildPayloadJSON := []byte(msg.OriginalJson())
	buildPayload := &common.InstanceTerminationPayload{}

	err := json.Unmarshal(buildPayloadJSON, buildPayload)
	if err != nil {
		log.WithField("err", err).Panic("failed to deserialize message")
	}

	err = newInstanceTerminatorWorker(buildPayload.InstanceID, buildPayload.SlackChannel,
		cfg, msg.Jid(), workers.Config.Pool.Get()).Terminate()
	if err != nil {
		log.WithField("err", err).Panic("instance build failed")
	}
}

type instanceTerminatorWorker struct {
	rc  redis.Conn
	jid string
	sc  string
	sn  *common.SlackNotifier
	iid string
	cfg *config
	ec2 *ec2.EC2
}

func newInstanceTerminatorWorker(instanceID, slackChannel string, cfg *config, jid string, redisConn redis.Conn) *instanceTerminatorWorker {
	return &instanceTerminatorWorker{
		rc:  redisConn,
		jid: jid,
		cfg: cfg,
		sc:  slackChannel,
		sn:  common.NewSlackNotifier(cfg.SlackTeam, cfg.SlackToken),
		iid: instanceID,
		ec2: ec2.New(cfg.AWSAuth, cfg.AWSRegion),
	}
}

func (itw *instanceTerminatorWorker) Terminate() error {
	_, err := itw.ec2.TerminateInstances([]string{itw.iid})
	if err != nil {
		return err
	}

	err = common.RemoveInstances(itw.rc, []string{itw.iid})
	if err != nil {
		itw.sn.Notify(itw.sc, fmt.Sprintf("Failed to terminate *%s* :scream_cat: _(%s)_", itw.iid, err))
		return err
	}

	itw.sn.Notify(itw.sc, fmt.Sprintf("Terminating *%s* :boom:", itw.iid))
	return nil
}
