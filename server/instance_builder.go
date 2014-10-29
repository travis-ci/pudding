package server

import (
	"encoding/json"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/travis-pro/worker-manager-service/common"
)

type instanceBuilder struct {
	QueueName string
	r         *redis.Pool
}

func newInstanceBuilder(redisURL, queueName string) (*instanceBuilder, error) {
	r, err := common.BuildRedisPool(redisURL)
	if err != nil {
		return nil, err
	}

	return &instanceBuilder{
		QueueName: queueName,

		r: r,
	}, nil
}

func (ib *instanceBuilder) Build(b *common.InstanceBuild) (*common.InstanceBuild, error) {
	conn := ib.r.Get()
	defer conn.Close()

	buildPayload := &common.InstanceBuildPayload{
		Class:      common.InstanceBuildClassname,
		Args:       []*common.InstanceBuild{b},
		Queue:      ib.QueueName,
		JID:        b.ID,
		Retry:      true,
		EnqueuedAt: float64(time.Now().UTC().Unix()),
	}

	buildPayloadJSON, err := json.Marshal(buildPayload)
	if err != nil {
		return nil, err
	}

	err = common.EnqueueJob(conn, ib.QueueName, string(buildPayloadJSON))
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

	err = conn.Send("DEL", common.InitScriptRedisKey(ID))
	if err != nil {
		conn.Send("DISCARD")
		return err
	}

	err = conn.Send("DEL", common.AuthRedisKey(ID))
	if err != nil {
		conn.Send("DISCARD")
		return err
	}

	return nil
}
