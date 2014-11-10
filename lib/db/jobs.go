package db

import (
	"fmt"

	"github.com/garyburd/redigo/redis"
	"github.com/travis-pro/pudding/lib"
)

// EnqueueJob pushes a given payload onto the given queue name to
// be consumed by the workers
func EnqueueJob(conn redis.Conn, queueName, payload string) error {
	err := conn.Send("MULTI")
	if err != nil {
		return err
	}
	err = conn.Send("SADD", fmt.Sprintf("%s:queues", lib.RedisNamespace), queueName)
	if err != nil {
		conn.Send("DISCARD")
		return err
	}

	err = conn.Send("LPUSH", fmt.Sprintf("%s:queue:%s", lib.RedisNamespace, queueName), payload)
	if err != nil {
		conn.Send("DISCARD")
		return err
	}

	_, err = conn.Do("EXEC")
	return err
}
