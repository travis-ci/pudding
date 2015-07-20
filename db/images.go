package db

import (
	"github.com/Sirupsen/logrus"
	"github.com/awslabs/aws-sdk-go/service/ec2"
	"github.com/garyburd/redigo/redis"
	"github.com/travis-ci/pudding"
)

// ImageFetcherStorer defines the interface for fetching and
// storing the internal image representation
type ImageFetcherStorer interface {
	Fetch(map[string]string) ([]*pudding.Image, error)
	Store(map[string]*ec2.Image) error
}

// Images represents the instance collection
type Images struct {
	Expiry int
	r      *redis.Pool
	log    *logrus.Logger
}

// NewImages creates a new Images collection
func NewImages(r *redis.Pool, log *logrus.Logger, expiry int) (*Images, error) {
	return &Images{
		Expiry: expiry,
		r:      r,
		log:    log,
	}, nil
}

// Fetch returns a slice of images, optionally with filter params
func (i *Images) Fetch(f map[string]string) ([]*pudding.Image, error) {
	conn := i.r.Get()
	defer conn.Close()

	return FetchImages(conn, f)
}

// Store accepts the ec2 representation of an image and stores it
func (i *Images) Store(images map[string]*ec2.Image) error {
	conn := i.r.Get()
	defer conn.Close()

	return StoreImages(conn, images, i.Expiry)
}
