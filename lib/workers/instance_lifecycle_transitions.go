package workers

import (
	"encoding/json"
	"fmt"

	"github.com/Sirupsen/logrus"
	"github.com/garyburd/redigo/redis"
	"github.com/goamz/goamz/autoscaling"
	"github.com/jrallison/go-workers"
	"github.com/travis-ci/pudding/lib"
	"github.com/travis-ci/pudding/lib/db"
)

var (
	errMissingInstanceLifecycleTransition = fmt.Errorf("missing instance lifecycle transition")
)

func init() {
	defaultQueueFuncs["instance-lifecycle-transitions"] = instanceLifecycleTransitionsMain
}

func instanceLifecycleTransitionsMain(cfg *internalConfig, msg *workers.Msg) {
	iltPayloadJSON := []byte(msg.OriginalJson())
	iltPayload := &lib.InstanceLifecycleTransitionPayload{
		Args: []*lib.InstanceLifecycleTransition{},
	}

	err := json.Unmarshal(iltPayloadJSON, iltPayload)
	if err != nil {
		log.WithField("err", err).Panic("failed to deserialize instance lifecycle transition")
	}

	ilt := iltPayload.InstanceLifecycleTransition()
	if ilt == nil {
		log.WithField("err", errMissingInstanceLifecycleTransition).Panic("no instance lifecycle transition available")
		return
	}

	err = handleInstanceLifecycleTransition(cfg, workers.Config.Pool.Get(), msg.Jid(), ilt)
	if err != nil {
		log.WithField("err", err).Panic("instance lifecycle transition handler returned an error")
	}
}

func handleInstanceLifecycleTransition(cfg *internalConfig, rc redis.Conn, jid string, ilt *lib.InstanceLifecycleTransition) error {
	// if instance transition set contains instance id
	//   complete lifecycle action with stored action token and hook name
	//   remove instance id from set
	//   remove instance id hash
	// else
	//   short circuit with log message

	ala, err := db.FetchInstanceLifecycleAction(rc, ilt.Transition, ilt.InstanceID)
	if err != nil {
		log.WithFields(logrus.Fields{
			"err":        err,
			"jid":        jid,
			"transition": ilt.Transition,
			"instance":   ilt.InstanceID,
		}).Error("failed to fetch instance lifecycle action")
		return err
	}

	if ala == nil {
		log.WithFields(logrus.Fields{
			"jid":        jid,
			"transition": ilt.Transition,
			"instance":   ilt.InstanceID,
		}).Warn("discarding unknown lifecycle transition")
		return nil
	}

	as := autoscaling.New(cfg.AWSAuth, cfg.AWSRegion)

	cla := &autoscaling.CompleteLifecycleActionParams{
		AutoScalingGroupName:  ala.AutoScalingGroupName,
		LifecycleActionResult: "CONTINUE",
		LifecycleActionToken:  ala.LifecycleActionToken,
		LifecycleHookName:     ala.LifecycleHookName,
	}

	_, err = as.CompleteLifecycleAction(cla)
	if err != nil {
		log.WithFields(logrus.Fields{
			"err":        err,
			"jid":        jid,
			"transition": ilt.Transition,
			"instance":   ilt.InstanceID,
		}).Error("failed to complete lifecycle action")
		return err
	}

	err = db.WipeInstanceLifecycleAction(rc, ilt.Transition, ilt.InstanceID)
	if err != nil {
		log.WithField("err", err).Warn("failed to clean up lifecycle action bits")
	}

	return nil
}
