package common

import (
	"fmt"
	"net/url"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/mitchellh/goamz/ec2"
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

func FetchInstances(conn redis.Conn) ([]*Instance, error) {
	keys, err := redis.Strings(conn.Do("SMEMBERS", fmt.Sprintf("%s:instances", RedisNamespace)))
	if err != nil {
		return nil, err
	}

	instances := []*Instance{}

	for _, key := range keys {
		reply, err := redis.Values(conn.Do("HGETALL", fmt.Sprintf("%s:instance:%s", RedisNamespace, key)))
		if err != nil {
			return nil, err
		}

		inst := &Instance{}
		err = redis.ScanStruct(reply, inst)
		if err != nil {
			return nil, err
		}

		// inst.ID = inst.InstanceID
		instances = append(instances, inst)
	}

	return instances, nil
}

func StoreInstances(conn redis.Conn, instances map[string]ec2.Instance, expiry int) error {
	err := conn.Send("MULTI")
	if err != nil {
		return err
	}

	instanceSetKey := fmt.Sprintf("%s:instances", RedisNamespace)

	err = conn.Send("DEL", instanceSetKey)
	if err != nil {
		conn.Do("DISCARD")
		return err
	}

	for ID, inst := range instances {
		instanceAttrsKey := fmt.Sprintf("%s:instance:%s", RedisNamespace, ID)

		err = conn.Send("SADD", instanceSetKey, ID)
		if err != nil {
			conn.Do("DISCARD")
			return err
		}

		hmSet := []interface{}{
			instanceAttrsKey,
			"instance_id", inst.InstanceId,
			"instance_type", inst.InstanceType,
			"image_id", inst.ImageId,
			"ip", inst.PublicIpAddress,
			"launch_time", inst.LaunchTime.Format(time.RFC3339),
		}

		for _, tag := range inst.Tags {
			switch tag.Key {
			case "queue", "env", "site":
				hmSet = append(hmSet, tag.Key, tag.Value)
			case "Name":
				hmSet = append(hmSet, "name", tag.Value)
			}
		}

		err = conn.Send("HMSET", hmSet...)
		if err != nil {
			conn.Do("DISCARD")
			return err
		}

		err = conn.Send("EXPIRE", instanceAttrsKey, expiry)
		if err != nil {
			conn.Do("DISCARD")
			return err
		}
	}

	err = conn.Send("EXPIRE", instanceSetKey, expiry)
	if err != nil {
		conn.Do("DISCARD")
		return err
	}

	_, err = conn.Do("EXEC")
	return err
}
