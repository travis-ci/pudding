package db

import (
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/goamz/goamz/ec2"
	"github.com/travis-ci/pudding"
)

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
func FetchInstances(conn redis.Conn, f map[string]string) ([]*pudding.Instance, error) {
	var err error
	keys := []string{}

	if key, ok := f["instance_id"]; ok {
		keys = append(keys, key)
	} else {
		keys, err = redis.Strings(conn.Do("SMEMBERS", fmt.Sprintf("%s:instances", pudding.RedisNamespace)))
		if err != nil {
			return nil, err
		}
	}

	instances := []*pudding.Instance{}

	for _, key := range keys {
		reply, err := redis.Values(conn.Do("HGETALL", fmt.Sprintf("%s:instance:%s", pudding.RedisNamespace, key)))
		if err != nil {
			return nil, err
		}

		inst := &pudding.Instance{}
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
			case "queue":
				if inst.Queue != value {
					failedChecks++
				}
			}
		}

		if failedChecks == 0 && !reflect.DeepEqual(inst, &pudding.Instance{}) {
			instances = append(instances, inst)
		}
	}

	return instances, nil
}

// SetInstanceAttributes sets key-value pair attributes on the
// given instance's hash
func SetInstanceAttributes(conn redis.Conn, instanceID string, attrs map[string]string) error {
	instanceAttrsKey := fmt.Sprintf("%s:instance:%s", pudding.RedisNamespace, instanceID)
	hmSet := []interface{}{instanceAttrsKey}
	for key, value := range attrs {
		hmSet = append(hmSet, key, value)
	}

	_, err := conn.Do("HMSET", hmSet...)
	return err
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

	instanceSetKey := fmt.Sprintf("%s:instances", pudding.RedisNamespace)

	err = conn.Send("DEL", instanceSetKey)
	if err != nil {
		conn.Do("DISCARD")
		return err
	}

	for ID, inst := range instances {
		instanceAttrsKey := fmt.Sprintf("%s:instance:%s", pudding.RedisNamespace, ID)

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
			"ip", inst.IPAddress,
			"private_ip", inst.PrivateIPAddress,
			"launch_time", inst.LaunchTime,
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

	instanceSetKey := fmt.Sprintf("%s:instances", pudding.RedisNamespace)

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

// FetchImages gets a slice of images given a redis conn and
// optional filter map
func FetchImages(conn redis.Conn, f map[string]string) ([]*pudding.Image, error) {
	var err error
	keys := []string{}

	if key, ok := f["image_id"]; ok {
		keys = append(keys, key)
	} else {
		keys, err = redis.Strings(conn.Do("SMEMBERS", fmt.Sprintf("%s:images", pudding.RedisNamespace)))
		if err != nil {
			return nil, err
		}
	}

	images := []*pudding.Image{}

	for _, key := range keys {
		reply, err := redis.Values(conn.Do("HGETALL", fmt.Sprintf("%s:image:%s", pudding.RedisNamespace, key)))
		if err != nil {
			return nil, err
		}

		img := &pudding.Image{}
		err = redis.ScanStruct(reply, img)
		if err != nil {
			return nil, err
		}

		failedChecks := 0
		for key, value := range f {
			switch key {
			case "active":
				if img.Active != (value == "true") {
					failedChecks++
				}
			case "role":
				if img.Role != value {
					failedChecks++
				}
			}
		}

		if failedChecks == 0 {
			images = append(images, img)
		}
	}

	return images, nil
}

// StoreImages stores the ec2 representation of an image
// given a redis conn and slice of ec2 images, as well as an
// expiry integer that is used to to run EXPIRE on all sets and
// hashes involved
func StoreImages(conn redis.Conn, images map[string]ec2.Image, expiry int) error {
	err := conn.Send("MULTI")
	if err != nil {
		return err
	}

	imageSetKey := fmt.Sprintf("%s:images", pudding.RedisNamespace)

	err = conn.Send("DEL", imageSetKey)
	if err != nil {
		conn.Do("DISCARD")
		return err
	}

	for ID, img := range images {
		imageAttrsKey := fmt.Sprintf("%s:image:%s", pudding.RedisNamespace, ID)

		err = conn.Send("SADD", imageSetKey, ID)
		if err != nil {
			conn.Do("DISCARD")
			return err
		}

		hmSet := []interface{}{
			imageAttrsKey,
			"image_id", img.Id,
			"name", img.Name,
			"state", img.State,
		}

		for _, tag := range img.Tags {
			switch tag.Key {
			case "role":
				hmSet = append(hmSet, tag.Key, tag.Value)
			case "active":
				hmSet = append(hmSet, tag.Key, true)
			}
		}

		err = conn.Send("HMSET", hmSet...)
		if err != nil {
			conn.Do("DISCARD")
			return err
		}

		err = conn.Send("EXPIRE", imageAttrsKey, expiry)
		if err != nil {
			conn.Do("DISCARD")
			return err
		}
	}

	err = conn.Send("EXPIRE", imageSetKey, expiry)
	if err != nil {
		conn.Do("DISCARD")
		return err
	}

	_, err = conn.Do("EXEC")
	return err
}

// RemoveImages removes the given images from the image
// set
func RemoveImages(conn redis.Conn, IDs []string) error {
	err := conn.Send("MULTI")
	if err != nil {
		return err
	}

	imageSetKey := fmt.Sprintf("%s:images", pudding.RedisNamespace)

	for _, ID := range IDs {
		err = conn.Send("SREM", imageSetKey, ID)
		if err != nil {
			conn.Do("DISCARD")
			return err
		}
	}

	_, err = conn.Do("EXEC")
	return err
}

// StoreInstanceLifecycleAction stores a pudding.AutoscalingLifecycleAction in a transition-specific set and hash
func StoreInstanceLifecycleAction(conn redis.Conn, a *pudding.AutoscalingLifecycleAction) error {
	err := conn.Send("MULTI")
	if err != nil {
		return err
	}

	transition := strings.ToLower(strings.Replace(a.LifecycleTransition, "autoscaling:EC2_INSTANCE_", "", 1))
	instSetKey := fmt.Sprintf("%s:instance_%s", pudding.RedisNamespace, transition)
	hashKey := fmt.Sprintf("%s:instance_%s:%s", pudding.RedisNamespace, transition, a.EC2InstanceID)

	err = conn.Send("SADD", instSetKey, a.EC2InstanceID)
	if err != nil {
		conn.Do("DISCARD")
		return err
	}

	hmSet := []interface{}{
		hashKey,
		"lifecycle_action_token", a.LifecycleActionToken,
		"auto_scaling_group_name", a.AutoScalingGroupName,
		"lifecycle_hook_name", a.LifecycleHookName,
	}

	err = conn.Send("HMSET", hmSet...)
	if err != nil {
		conn.Do("DISCARD")
		return err
	}

	_, err = conn.Do("EXEC")
	return err
}

// FetchInstanceLifecycleAction retrieves a pudding.AutoscalingLifecycleAction
func FetchInstanceLifecycleAction(conn redis.Conn, transition, instanceID string) (*pudding.AutoscalingLifecycleAction, error) {
	exists, err := redis.Bool(conn.Do("SISMEMBER", fmt.Sprintf("%s:instance_%s", pudding.RedisNamespace, transition), instanceID))
	if !exists {
		return nil, nil
	}

	attrs, err := redis.Values(conn.Do("HGETALL", fmt.Sprintf("%s:instance_%s:%s", pudding.RedisNamespace, transition, instanceID)))
	if err != nil {
		return nil, err
	}

	ala := &pudding.AutoscalingLifecycleAction{}
	err = redis.ScanStruct(attrs, ala)
	return ala, err
}

// WipeInstanceLifecycleAction cleans up the keys for a given lifecycle action
func WipeInstanceLifecycleAction(conn redis.Conn, transition, instanceID string) error {
	err := conn.Send("MULTI")
	if err != nil {
		return err
	}

	err = conn.Send("SREM", fmt.Sprintf("%s:instance_%s", pudding.RedisNamespace, transition), instanceID)
	if err != nil {
		conn.Do("DISCARD")
		return err
	}

	err = conn.Send("DEL", fmt.Sprintf("%s:instance_%s:%s", pudding.RedisNamespace, transition, instanceID))
	if err != nil {
		conn.Do("DISCARD")
		return err
	}

	_, err = conn.Do("EXEC")
	return err
}
