package workers

import (
	"github.com/Sirupsen/logrus"
	"github.com/mitchellh/goamz/ec2"
	"github.com/travis-pro/pudding/lib"
	"github.com/travis-pro/pudding/lib/db"
)

type ec2Syncer struct {
	cfg *internalConfig
	ec2 *ec2.EC2
	log *logrus.Logger
	i   db.InstanceFetcherStorer
}

func newEC2Syncer(cfg *internalConfig, log *logrus.Logger) (*ec2Syncer, error) {
	i, err := db.NewInstances(cfg.RedisURL.String(), log, cfg.InstanceStoreExpiry)
	if err != nil {
		return nil, err
	}

	return &ec2Syncer{
		cfg: cfg,
		log: log,
		i:   i,
		ec2: ec2.New(cfg.AWSAuth, cfg.AWSRegion),
	}, nil
}

func (es *ec2Syncer) Sync() error {
	es.log.Debug("ec2 syncer fetching worker instances")
	instances, err := lib.GetWorkerInstances(es.ec2)
	if err != nil {
		panic(err)
	}

	es.log.Debug("ec2 syncer storing instances")
	err = es.i.Store(instances)
	if err != nil {
		panic(err)
	}

	return nil
}
