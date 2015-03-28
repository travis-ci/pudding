package server

import (
	"encoding/json"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/travis-ci/pudding"
	"github.com/travis-ci/pudding/db"
)

type snsHandler struct {
	QueueName string
	r         *redis.Pool
}

func newSNSHandler(r *redis.Pool, queueName string) (*snsHandler, error) {
	return &snsHandler{
		QueueName: queueName,
		r:         r,
	}, nil
}

func (sh *snsHandler) Handle(msg *pudding.SNSMessage) (*pudding.SNSMessage, error) {
	conn := sh.r.Get()
	defer func() { _ = conn.Close() }()

	messagePayload := &pudding.SNSMessagePayload{
		Args:       []*pudding.SNSMessage{msg},
		Queue:      sh.QueueName,
		JID:        msg.MessageID,
		Retry:      true,
		EnqueuedAt: float64(time.Now().UTC().Unix()),
	}

	messagePayloadJSON, err := json.Marshal(messagePayload)
	if err != nil {
		return nil, err
	}

	err = db.EnqueueJob(conn, sh.QueueName, string(messagePayloadJSON))
	return msg, err
}
