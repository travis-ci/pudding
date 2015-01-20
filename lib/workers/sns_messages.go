package workers

import (
	"encoding/json"
	"fmt"

	"github.com/garyburd/redigo/redis"
	"github.com/jrallison/go-workers"
	"github.com/travis-ci/pudding/lib"
	"github.com/travis-ci/pudding/lib/db"
)

var (
	errMissingSNSMessage = fmt.Errorf("missing sns message")
	snsMessageHandlers   = map[string]func(redis.Conn, *lib.SNSMessage) error{
		"SubscriptionConfirmation": func(rc redis.Conn, msg *lib.SNSMessage) error {
			log.WithField("msg", msg).Info("subscription confirmation not really being handled")
			return nil
		},
		"Notification": handleSNSNotification,
	}
)

func init() {
	defaultQueueFuncs["sns-messages"] = snsMessagesMain
}

func snsMessagesMain(cfg *internalConfig, msg *workers.Msg) {
	snsMessagePayloadJSON := []byte(msg.OriginalJson())
	snsMessagePayload := &lib.SNSMessagePayload{
		Args: []*lib.SNSMessage{},
	}

	err := json.Unmarshal(snsMessagePayloadJSON, snsMessagePayload)
	if err != nil {
		log.WithField("err", err).Panic("failed to deserialize message")
	}

	snsMsg := snsMessagePayload.SNSMessage()
	if snsMsg == nil {
		log.WithField("err", errMissingSNSMessage).Panic("no sns message available")
		return
	}

	handlerFunc, ok := snsMessageHandlers[snsMsg.Type]
	if !ok {
		log.WithField("type", snsMsg.Type).Warn("no handler available for message type")
		return
	}

	err = handlerFunc(workers.Config.Pool.Get(), snsMsg)
	if err != nil {
		log.WithField("err", err).Panic("sns handler returned an error")
	}
}

func handleSNSNotification(rc redis.Conn, msg *lib.SNSMessage) error {
	log.WithField("msg", msg).Info("received an SNS notification")

	a, err := msg.AutoscalingLifecycleAction()
	if err != nil {
		log.WithField("err", err).Warn("unable to handle notification")
		return nil
	}

	if a.Event == "autoscaling:TEST_NOTIFICATION" {
		log.WithField("event", a.Event).Info("ignoring")
		return nil
	}

	switch a.LifecycleTransition {
	case "autoscaling:EC2_INSTANCE_LAUNCHING":
		return db.StoreInstanceLifecycleAction(rc, a)
	case "autoscaling:EC2_INSTANCE_TERMINATING":
		err = db.SetInstanceAttributes(rc, a.EC2InstanceID, map[string]string{"expected_state": "down"})
		if err != nil {
			return err
		}
		return db.StoreInstanceLifecycleAction(rc, a)
	default:
		log.WithField("action", a).Warn("unable to handle unknown lifecycle transition")
	}

	return nil
}
