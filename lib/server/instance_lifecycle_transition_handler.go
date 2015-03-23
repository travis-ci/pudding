package server

import (
	"encoding/json"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/travis-ci/pudding/lib"
	"github.com/travis-ci/pudding/lib/db"
)

type instanceLifecycleTransitionHandler struct {
	QueueName string
	r         *redis.Pool
}

func newInstanceLifecycleTransitionHandler(r *redis.Pool, queueName string) (*instanceLifecycleTransitionHandler, error) {
	return &instanceLifecycleTransitionHandler{
		QueueName: queueName,
		r:         r,
	}, nil
}

func (th *instanceLifecycleTransitionHandler) Handle(t *lib.InstanceLifecycleTransition) (*lib.InstanceLifecycleTransition, error) {
	conn := th.r.Get()
	defer func() { _ = conn.Close() }()

	messagePayload := &lib.InstanceLifecycleTransitionPayload{
		Args:       []*lib.InstanceLifecycleTransition{t},
		Queue:      th.QueueName,
		JID:        t.ID,
		Retry:      true,
		EnqueuedAt: float64(time.Now().UTC().Unix()),
	}

	messagePayloadJSON, err := json.Marshal(messagePayload)
	if err != nil {
		return nil, err
	}

	err = db.EnqueueJob(conn, th.QueueName, string(messagePayloadJSON))
	return t, err
}
