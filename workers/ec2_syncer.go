package workers

import (
	"fmt"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/garyburd/redigo/redis"
	"github.com/mitchellh/goamz/ec2"
	"github.com/travis-pro/worker-manager-service/common"
)

type ec2Syncer struct {
	cfg *config
	r   *redis.Pool
	ec2 *ec2.EC2
	log *logrus.Logger
}

func newEC2Syncer(cfg *config, log *logrus.Logger) (*ec2Syncer, error) {
	r, err := common.BuildRedisPool(cfg.RedisURL.String())
	if err != nil {
		return nil, err
	}

	return &ec2Syncer{
		cfg: cfg,
		log: log,
		r:   r,
		ec2: ec2.New(cfg.AWSAuth, cfg.AWSRegion),
	}, nil
}

func (es *ec2Syncer) Sync() error {
	es.log.Debug("ec2 syncer fetching worker instances")
	instances, err := es.getWorkerInstances()
	if err != nil {
		return err
	}

	es.log.Debug("ec2 syncer storing instances")
	err = es.storeInstances(instances)
	if err != nil {
		return err
	}

	return nil
}

func (es *ec2Syncer) storeInstances(instances map[string]ec2.Instance) error {
	conn := es.r.Get()
	defer conn.Close()

	err := conn.Send("MULTI")
	if err != nil {
		return err
	}

	instanceSetKey := fmt.Sprintf("%s:instances", common.RedisNamespace)

	err = conn.Send("DEL", instanceSetKey)
	if err != nil {
		conn.Do("DISCARD")
		return err
	}

	for ID, inst := range instances {
		instanceAttrsKey := fmt.Sprintf("%s:instance:%s", common.RedisNamespace, ID)

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

		err = conn.Send("EXPIRE", instanceAttrsKey, es.cfg.MiniWorkerInterval*3)
		if err != nil {
			conn.Do("DISCARD")
			return err
		}
	}

	_, err = conn.Do("EXEC")
	return err
}

func (es *ec2Syncer) getWorkerInstances() (map[string]ec2.Instance, error) {
	filter := ec2.NewFilter()
	filter.Add("tag:role", "worker")
	resp, err := es.ec2.Instances([]string{}, filter)

	if err != nil {
		return nil, err
	}

	instances := map[string]ec2.Instance{}

	for _, res := range resp.Reservations {
		for _, inst := range res.Instances {
			instances[inst.InstanceId] = inst
		}
	}

	return instances, nil
}
