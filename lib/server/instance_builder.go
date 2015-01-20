package server

import (
	"encoding/json"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/travis-ci/pudding/lib"
	"github.com/travis-ci/pudding/lib/db"
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

func (ib *instanceBuilder) Build(b *lib.InstanceBuild) (*lib.InstanceBuild, error) {
	conn := ib.r.Get()
	defer conn.Close()

	buildPayload := &lib.InstanceBuildPayload{
		Args:       []*lib.InstanceBuild{b},
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
