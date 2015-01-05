package workers

import (
	"encoding/json"
	"fmt"

	"github.com/jrallison/go-workers"
	"github.com/travis-ci/pudding/lib"
)

var (
	errMissingSNSMessage = fmt.Errorf("missing sns message")
	snsMessageHandlers   = map[string]func(*lib.SNSMessage) error{
		"SubscriptionConfirmation": func(msg *lib.SNSMessage) error {
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
		Args: []*lib.SNSMessage{
			&lib.SNSMessage{},
		},
	}

	err := json.Unmarshal(snsMessagePayloadJSON, snsMessagePayload)
	if err != nil {
		log.WithField("err", err).Panic("failed to deserialize message")
	}

	snsMsg := snsMessagePayload.SNSMessage()
	if snsMsg == nil {
		log.WithField("err", errMissingSNSMessage).Panic("no sns message available")
	}

	handlerFunc, ok := snsMessageHandlers[snsMsg.Type]
	if !ok {
		log.WithField("type", snsMsg.Type).Warn("no handler available for message type")
		return
	}

	err = handlerFunc(snsMsg)
	if err != nil {
		log.WithField("err", err).Panic("sns handler returned an error")
	}
}

func handleSNSNotification(msg *lib.SNSMessage) error {
	log.WithField("msg", msg).Info("received an SNS notification")
	return nil
}
