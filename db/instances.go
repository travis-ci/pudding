package db

import (
	"github.com/Sirupsen/logrus"
	"github.com/garyburd/redigo/redis"
	"github.com/goamz/goamz/ec2"
	"github.com/travis-ci/pudding/lib"
)

// InstanceFetcherStorer defines the interface for fetching and
// storing the internal instance representation
type InstanceFetcherStorer interface {
	Fetch(map[string]string) ([]*pudding.Instance, error)
	Store(map[string]ec2.Instance) error
}

// Instances represents the instance collection
type Instances struct {
	Expiry int
	r      *redis.Pool
	log    *logrus.Logger
}

// NewInstances creates a new Instances collection
func NewInstances(r *redis.Pool, log *logrus.Logger, expiry int) (*Instances, error) {
	return &Instances{
		Expiry: expiry,
		r:      r,
		log:    log,
	}, nil
}

// Fetch returns a slice of instances, optionally with filter params
func (i *Instances) Fetch(f map[string]string) ([]*pudding.Instance, error) {
	conn := i.r.Get()
	defer conn.Close()

	return FetchInstances(conn, f)
}

// Store accepts the ec2 representation of an instance and stores it
func (i *Instances) Store(instances map[string]ec2.Instance) error {
	conn := i.r.Get()
	defer conn.Close()

	return StoreInstances(conn, instances, i.Expiry)
}
