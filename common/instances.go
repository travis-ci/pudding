package common

import (
	"github.com/Sirupsen/logrus"
	"github.com/garyburd/redigo/redis"
	"github.com/mitchellh/goamz/ec2"
)

type InstanceFetcherStorer interface {
	Fetch(map[string]string) ([]*Instance, error)
	Store(map[string]ec2.Instance) error
}

type Instances struct {
	Expiry int
	r      *redis.Pool
	log    *logrus.Logger
}

func NewInstances(redisURL string, log *logrus.Logger, expiry int) (*Instances, error) {
	r, err := BuildRedisPool(redisURL)
	if err != nil {
		return nil, err
	}

	return &Instances{
		Expiry: expiry,
		r:      r,
		log:    log,
	}, nil
}

func (i *Instances) Fetch(f map[string]string) ([]*Instance, error) {
	conn := i.r.Get()
	defer conn.Close()

	return FetchInstances(conn, f)
}

func (i *Instances) Store(instances map[string]ec2.Instance) error {
	conn := i.r.Get()
	defer conn.Close()

	return StoreInstances(conn, instances, i.Expiry)
}
