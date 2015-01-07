package server

import (
	"encoding/json"

	"github.com/garyburd/redigo/redis"
	"github.com/travis-ci/pudding/lib"
	"github.com/travis-ci/pudding/lib/db"
)

type instanceTerminator struct {
	QueueName string
	r         *redis.Pool
}

func newInstanceTerminator(r *redis.Pool, queueName string) (*instanceTerminator, error) {
	return &instanceTerminator{
		QueueName: queueName,

		r: r,
	}, nil
}

func (it *instanceTerminator) Terminate(instanceID, slackChannel string) error {
	conn := it.r.Get()
	defer conn.Close()

	buildPayload := &lib.InstanceTerminationPayload{
		InstanceID:   instanceID,
		SlackChannel: slackChannel,
	}

	buildPayloadJSON, err := json.Marshal(buildPayload)
	if err != nil {
		return err
	}

	return db.EnqueueJob(conn, it.QueueName, string(buildPayloadJSON))
}
