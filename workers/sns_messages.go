package workers

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sns"
	"github.com/garyburd/redigo/redis"
	"github.com/jrallison/go-workers"
	"github.com/travis-ci/pudding"
	"github.com/travis-ci/pudding/db"
)

var (
	errMissingSNSMessage = fmt.Errorf("missing sns message")
	snsMessageHandlers   = map[string]func(redis.Conn, *pudding.SNSMessage) error{
		"SubscriptionConfirmation": handleSNSConfirmation,
		"Notification":             handleSNSNotification,
	}
)

func init() {
	defaultQueueFuncs["sns-messages"] = snsMessagesMain
}

func snsMessagesMain(cfg *internalConfig, msg *workers.Msg) {
	snsMessagePayloadJSON := []byte(msg.OriginalJson())
	snsMessagePayload := &pudding.SNSMessagePayload{
		Args: []*pudding.SNSMessage{},
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

// http://docs.aws.amazon.com/sns/latest/dg/SendMessageToHttp.html
func handleSNSConfirmation(rc redis.Conn, msg *pudding.SNSMessage) error {
	if v, _ := strconv.ParseBool(os.Getenv("SNS_CONFIRMATION")); v {
		log.WithField("msg", msg).Info("handling subscription confirmation")

		svc := sns.New(session.New())

		params := &sns.ConfirmSubscriptionInput{
			Token:    aws.String(msg.Token),
			TopicArn: aws.String(msg.TopicARN),
		}
		resp, err := svc.ConfirmSubscription(params)
		if err != nil {
			return err
		}

		log.WithField("subscription", resp.String()).Info("confirmed subscription")

		return nil
	}

	log.WithField("msg", msg).Info("subscription confirmation not really being handled")

	return nil
}

func handleSNSNotification(rc redis.Conn, msg *pudding.SNSMessage) error {
	log.WithField("msg", msg).Debug("received an SNS notification")

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
		log.WithField("action", a).Debug("storing instance launching lifecycle action")
		return db.StoreInstanceLifecycleAction(rc, a)
	case "autoscaling:EC2_INSTANCE_TERMINATING":
		log.WithField("action", a).Debug("setting expected_state to down")
		err = db.SetInstanceAttributes(rc, a.EC2InstanceID, map[string]string{"expected_state": "down"})
		if err != nil {
			return err
		}
		log.WithField("action", a).Debug("storing instance terminating lifecycle action")
		return db.StoreInstanceLifecycleAction(rc, a)
	default:
		log.WithField("action", a).Warn("unable to handle unknown lifecycle transition")
	}

	return nil
}
