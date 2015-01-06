package workers

import (
	"net"
	"net/url"

	"github.com/Sirupsen/logrus"
	"github.com/goamz/goamz/ec2"
	"github.com/travis-ci/pudding/lib"
	"github.com/travis-ci/pudding/lib/db"
)

type ec2Syncer struct {
	cfg *internalConfig
	ec2 *ec2.EC2
	log *logrus.Logger
	i   db.InstanceFetcherStorer
	img db.ImageFetcherStorer
}

func newEC2Syncer(cfg *internalConfig, log *logrus.Logger) (*ec2Syncer, error) {
	i, err := db.NewInstances(cfg.RedisURL.String(), log, cfg.InstanceStoreExpiry)
	if err != nil {
		return nil, err
	}

	img, err := db.NewImages(cfg.RedisURL.String(), log, cfg.ImageStoreExpiry)
	if err != nil {
		return nil, err
	}

	return &ec2Syncer{
		cfg: cfg,
		log: log,
		i:   i,
		img: img,
		ec2: ec2.New(cfg.AWSAuth, cfg.AWSRegion),
	}, nil
}

func (es *ec2Syncer) Sync() error {
	var (
		instances map[string]ec2.Instance
		images    map[string]ec2.Image
		err       error
	)

	es.log.Debug("ec2 syncer fetching instances")
	for i := 3; i > 0; i-- {
		instances, err = es.fetchInstances()
		if err == nil {
			break
		}
	}

	if err != nil {
		panic(err)
	}

	if instances == nil {
		es.log.Debug("ec2 syncer failed to get any instances; assuming temporary network error")
		return nil
	}

	es.log.Debug("ec2 syncer storing instances")
	err = es.i.Store(instances)
	if err != nil {
		panic(err)
	}

	es.log.Debug("ec2 syncer fetching images")
	for i := 3; i > 0; i-- {
		images, err = es.fetchImages()
		if err == nil {
			break
		}
	}

	if err != nil {
		panic(err)
	}

	if images == nil {
		es.log.Debug("ec2 syncer failed to get any images; assuming temporary network error")
		return nil
	}

	es.log.Debug("ec2 syncer storing images")
	err = es.img.Store(images)
	if err != nil {
		panic(err)
	}

	return nil
}

func (es *ec2Syncer) fetchInstances() (map[string]ec2.Instance, error) {
	f := ec2.NewFilter()
	f.Add("instance-state-name", "running")
	instances, err := lib.GetInstancesWithFilter(es.ec2, f)
	if err == nil {
		return instances, nil
	}

	switch err.(type) {
	case *url.Error, *net.OpError:
		log.WithFields(logrus.Fields{"err": err}).Warn("network error while fetching ec2 instances")
		return nil, nil
	default:
		return nil, err
	}
}

func (es *ec2Syncer) fetchImages() (map[string]ec2.Image, error) {
	f := ec2.NewFilter()
	f.Add("tag-key", "role")
	images, err := lib.GetImagesWithFilter(es.ec2, f)
	if err == nil {
		return images, nil
	}

	switch err.(type) {
	case *url.Error, *net.OpError:
		log.WithFields(logrus.Fields{"err": err}).Warn("network error while fetching ec2 images")
		return nil, nil
	default:
		return nil, err
	}
}
