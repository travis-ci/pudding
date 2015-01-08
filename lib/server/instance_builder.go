package server

import (
	"encoding/json"
	"fmt"
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

func (ib *instanceBuilder) Wipe(ID string) error {
	conn := ib.r.Get()
	defer conn.Close()

	err := conn.Send("MULTI")
	if err != nil {
		conn.Send("DISCARD")
		return err
	}

	err = conn.Send("HDEL", fmt.Sprintf("%s:init-scripts", lib.RedisNamespace), ID)
	if err != nil {
		conn.Send("DISCARD")
		return err
	}

	err = conn.Send("HDEL", fmt.Sprintf("%s:auths", lib.RedisNamespace), ID)
	if err != nil {
		conn.Send("DISCARD")
		return err
	}

	return nil
}
