package server

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/travis-pro/worker-manager-service/common"
)

type instanceBuilder struct {
	redisURLString string
	QueueName      string
	RedisNamespace string
	r              *redis.Pool
}

func newInstanceBuilder(redisURL, queueName string) (*instanceBuilder, error) {
	ib := &instanceBuilder{
		redisURLString: redisURL,
		QueueName:      queueName,
		RedisNamespace: common.RedisNamespace,
	}
	err := ib.Setup()
	if err != nil {
		return nil, err
	}

	return ib, nil
}

func (ib *instanceBuilder) Setup() error {
	pool, err := common.BuildRedisPool(ib.redisURLString)
	if err != nil {
		return err
	}

	ib.r = pool
	return nil
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

	err = conn.Send("MULTI")
	if err != nil {
		return nil, err
	}
	err = conn.Send("SADD", fmt.Sprintf("%s:queues", ib.RedisNamespace), ib.QueueName)
	if err != nil {
		conn.Send("DISCARD")
		return nil, err
	}

	err = conn.Send("LPUSH", fmt.Sprintf("%s:queue:%s", ib.RedisNamespace, ib.QueueName), buildPayloadJSON)
	if err != nil {
		conn.Send("DISCARD")
		return nil, err
	}

	_, err = conn.Do("EXEC")
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
