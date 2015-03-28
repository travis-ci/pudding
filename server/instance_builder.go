package server

import (
	"encoding/json"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/travis-ci/pudding"
	"github.com/travis-ci/pudding/db"
)

type instanceBuilder struct {
	QueueName string
	r         *redis.Pool
}

func newInstanceBuilder(r *redis.Pool, queueName string) (*instanceBuilder, error) {
	return &instanceBuilder{
		QueueName: queueName,

		r: r,
	}, nil
}

func (ib *instanceBuilder) Build(b *pudding.InstanceBuild) (*pudding.InstanceBuild, error) {
	conn := ib.r.Get()
	defer func() { _ = conn.Close() }()

	buildPayload := &pudding.InstanceBuildPayload{
		Args:       []*pudding.InstanceBuild{b},
		Queue:      ib.QueueName,
		JID:        b.ID,
		Retry:      true,
		EnqueuedAt: float64(time.Now().UTC().Unix()),
	}

	buildPayloadJSON, err := json.Marshal(buildPayload)
	if err != nil {
		return nil, err
	}

	err = db.EnqueueJob(conn, ib.QueueName, string(buildPayloadJSON))
	return b, err
}
