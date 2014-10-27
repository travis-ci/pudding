package common

import (
	"fmt"
	"net/url"
	"time"

	"github.com/garyburd/redigo/redis"
)

func InitScriptRedisKey(instanceBuildID string) string {
	return fmt.Sprintf("worker-manager:init-script:%s", instanceBuildID)
}

func AuthRedisKey(instanceBuildID string) string {
	return fmt.Sprintf("worker-manager:auth:%s", instanceBuildID)
}

func BuildRedisPool(redisURL string) (*redis.Pool, error) {
	u, err := url.Parse(redisURL)
	if err != nil {
		return nil, err
	}

	pool := &redis.Pool{
		MaxIdle:     3,
		IdleTimeout: 240 * time.Second,
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", u.Host)
			if err != nil {
				return nil, err
			}
			if u.User == nil {
				return c, err
			}
			if auth, ok := u.User.Password(); ok {
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
	return pool, nil
}
