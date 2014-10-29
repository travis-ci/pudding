package server

import (
	"encoding/json"

	"github.com/garyburd/redigo/redis"
	"github.com/travis-pro/worker-manager-service/lib"
	"github.com/travis-pro/worker-manager-service/lib/db"
)

type instanceTerminator struct {
	QueueName string
	r         *redis.Pool
}

func newInstanceTerminator(redisURL, queueName string) (*instanceTerminator, error) {
	r, err := db.BuildRedisPool(redisURL)
	if err != nil {
		return nil, err
	}

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
