package server

import (
	"encoding/json"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/travis-pro/pudding/lib"
	"github.com/travis-pro/pudding/lib/db"
)

type instanceBuilder struct {
	QueueName string
	r         *redis.Pool
}

func newInstanceBuilder(redisURL, queueName string) (*instanceBuilder, error) {
	r, err := db.BuildRedisPool(redisURL)
	if err != nil {
		return nil, err
	}

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

	err = conn.Send("DEL", db.InitScriptRedisKey(ID))
	if err != nil {
		conn.Send("DISCARD")
		return err
	}

	err = conn.Send("DEL", db.AuthRedisKey(ID))
	if err != nil {
		conn.Send("DISCARD")
		return err
	}

	return nil
}
