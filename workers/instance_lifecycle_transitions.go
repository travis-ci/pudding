package workers

import (
	"encoding/json"
	"fmt"

	"github.com/Sirupsen/logrus"
	"github.com/awslabs/aws-sdk-go/aws"
	"github.com/awslabs/aws-sdk-go/service/autoscaling"
	"github.com/garyburd/redigo/redis"
	"github.com/jrallison/go-workers"
	"github.com/travis-ci/pudding"
	"github.com/travis-ci/pudding/db"
)

var (
	errMissingInstanceLifecycleTransition = fmt.Errorf("missing instance lifecycle transition")
)

func init() {
	defaultQueueFuncs["instance-lifecycle-transitions"] = instanceLifecycleTransitionsMain
}

func instanceLifecycleTransitionsMain(cfg *internalConfig, msg *workers.Msg) {
	iltPayloadJSON := []byte(msg.OriginalJson())
	iltPayload := &pudding.InstanceLifecycleTransitionPayload{
		Args: []*pudding.InstanceLifecycleTransition{},
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
	awsErr := aws.Error(err)
	if awsErr != nil || err != nil {
		if awsErr != nil {
			log.WithField("err", err).Error("discarding autoscaling error")
		} else {
			log.WithField("err", err).Panic("instance lifecycle transition handler returned an error")
		}
	}
}

func handleInstanceLifecycleTransition(cfg *internalConfig, rc redis.Conn, jid string, ilt *pudding.InstanceLifecycleTransition) error {
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

	as := autoscaling.New(cfg.AWSConfig)

	cla := &autoscaling.CompleteLifecycleActionInput{
		AutoScalingGroupName:  aws.String(ala.AutoScalingGroupName),
		LifecycleActionResult: aws.String("CONTINUE"),
		LifecycleActionToken:  aws.String(ala.LifecycleActionToken),
		LifecycleHookName:     aws.String(ala.LifecycleHookName),
	}

	log.WithFields(logrus.Fields{
		"jid":        jid,
		"transition": ilt.Transition,
		"instance":   ilt.InstanceID,
		"params":     fmt.Sprintf("%#v", cla),
	}).Info("completing lifecycle action")

	_, err = as.CompleteLifecycleAction(cla)
	if err != nil {
		log.WithFields(logrus.Fields{
			"err":        err,
			"jid":        jid,
			"transition": ilt.Transition,
			"instance":   ilt.InstanceID,
			"params":     fmt.Sprintf("%#v", cla),
		}).Error("failed to complete lifecycle action")
		return err
	}

	err = db.WipeInstanceLifecycleAction(rc, ilt.Transition, ilt.InstanceID)
	if err != nil {
		log.WithField("err", err).Warn("failed to clean up lifecycle action bits")
	}

	return nil
}
