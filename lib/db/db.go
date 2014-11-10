package db

import (
	"fmt"
	"net/url"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/mitchellh/goamz/ec2"
	"github.com/travis-ci/pudding/lib"
)

// InitScriptRedisKey provides the key for an init script given the
// instance build id
func InitScriptRedisKey(instanceBuildID string) string {
	return fmt.Sprintf("%s:init-script:%s", lib.RedisNamespace, instanceBuildID)
}

// AuthRedisKey provides the auth key given an instance build id
func AuthRedisKey(instanceBuildID string) string {
	return fmt.Sprintf("%s:auth:%s", lib.RedisNamespace, instanceBuildID)
}

// BuildRedisPool builds a *redis.Pool given a redis URL yey ☃
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

// FetchInstances gets a slice of instances given a redis conn and
// optional filter map
func FetchInstances(conn redis.Conn, f map[string]string) ([]*lib.Instance, error) {
	var err error
	keys := []string{}

	if key, ok := f["instance_id"]; ok {
		keys = append(keys, key)
	} else {
		keys, err = redis.Strings(conn.Do("SMEMBERS", fmt.Sprintf("%s:instances", lib.RedisNamespace)))
		if err != nil {
			return nil, err
		}
	}

	instances := []*lib.Instance{}

	for _, key := range keys {
		reply, err := redis.Values(conn.Do("HGETALL", fmt.Sprintf("%s:instance:%s", lib.RedisNamespace, key)))
		if err != nil {
			return nil, err
		}

		inst := &lib.Instance{}
		err = redis.ScanStruct(reply, inst)
		if err != nil {
			return nil, err
		}

		failedChecks := 0
		for key, value := range f {
			switch key {
			case "env":
				if inst.Env != value {
					failedChecks++
				}
			case "site":
				if inst.Site != value {
					failedChecks++
				}
			case "role":
				if inst.Role != value {
					failedChecks++
				}
			}
		}

		if failedChecks == 0 {
			instances = append(instances, inst)
		}
	}

	return instances, nil
}

// StoreInstances stores the ec2 representation of an instance
// given a redis conn and slice of ec2 instances, as well as an
// expiry integer that is used to to run EXPIRE on all sets and
// hashes involved
func StoreInstances(conn redis.Conn, instances map[string]ec2.Instance, expiry int) error {
	err := conn.Send("MULTI")
	if err != nil {
		return err
	}

	instanceSetKey := fmt.Sprintf("%s:instances", lib.RedisNamespace)

	err = conn.Send("DEL", instanceSetKey)
	if err != nil {
		conn.Do("DISCARD")
		return err
	}

	for ID, inst := range instances {
		instanceAttrsKey := fmt.Sprintf("%s:instance:%s", lib.RedisNamespace, ID)

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
			"private_ip", inst.PrivateIpAddress,
			"launch_time", inst.LaunchTime.Format(time.RFC3339),
		}

		for _, tag := range inst.Tags {
			switch tag.Key {
			case "queue", "env", "site", "role":
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

// RemoveInstances removes the given instances from the instance
// set
func RemoveInstances(conn redis.Conn, IDs []string) error {
	err := conn.Send("MULTI")
	if err != nil {
		return err
	}

	instanceSetKey := fmt.Sprintf("%s:instances", lib.RedisNamespace)

	for _, ID := range IDs {
		err = conn.Send("SREM", instanceSetKey, ID)
		if err != nil {
			conn.Do("DISCARD")
			return err
		}
	}

	_, err = conn.Do("EXEC")
	return err
}
