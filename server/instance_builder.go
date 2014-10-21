package server

import (
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/gorilla/feeds"
	"github.com/travis-pro/worker-manager-service/common"
)

type instanceBuilder struct {
	RedisURL       *url.URL
	QueueName      string
	RedisNamespace string
	r              *redis.Pool
}

type instanceBuildDetails struct {
	ID string
}

type instanceBuildPayload struct {
	Class      string        `json:"class"`
	Args       []interface{} `json:"args"`
	Queue      string        `json:"queue,omitempty"`
	JID        string        `json:"jid,omitempty"`
	RetryCount string        `json:"retry_count,omitempty"`
	FailedAt   float64       `json:"failed_at,omitempty"`
}

func newInstanceBuilder(redisURL, queueName string) (*instanceBuilder, error) {
	u, err := url.Parse(redisURL)
	if err != nil {
		return nil, err
	}

	ib := &instanceBuilder{
		RedisURL:       u,
		QueueName:      queueName,
		RedisNamespace: "worker-manager",
	}
	ib.Setup()
	return ib, nil
}

func (ib *instanceBuilder) Setup() {
	ib.buildRedisPool()
}

func (ib *instanceBuilder) buildRedisPool() {
	ib.r = &redis.Pool{
		MaxIdle:     3,
		IdleTimeout: 240 * time.Second,
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", ib.RedisURL.Host)
			if err != nil {
				return nil, err
			}
			if ib.RedisURL.User == nil {
				return c, err
			}
			if auth, ok := ib.RedisURL.User.Password(); ok {
				if _, err := c.Do("AUTH", auth); err != nil {
					c.Close()
					return nil, err
				}
			}
			return c, err
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			_, err := c.Do("PING")
			return err
		},
	}
}

func (ib *instanceBuilder) Build(b *instanceBuild) (*instanceBuildDetails, error) {
	conn := ib.r.Get()
	defer conn.Close()

	_, err := conn.Do("PING")
	if err != nil {
		return nil, err
	}

	d := &instanceBuildDetails{}
	d.ID = feeds.NewUUID().String()

	buildPayload := &instanceBuildPayload{
		Class: common.InstanceBuildClassname,
		Args:  []interface{}{d.ID, b},
		Queue: ib.QueueName,
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
		return nil, err
	}

	err = conn.Send("LPUSH", fmt.Sprintf("%s:queue:%s", ib.RedisNamespace, ib.QueueName), buildPayloadJSON)
	if err != nil {
		return nil, err
	}

	_, err = conn.Do("EXEC")
	return d, err
}
