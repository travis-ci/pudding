package workers

import (
	"github.com/Sirupsen/logrus"
	"github.com/benmanns/goworker"
	"github.com/travis-pro/worker-manager-service/common"
)

func init() {
	goworker.Register(common.InstanceBuildClassname, instanceBuildsMain)
}

func instanceBuildsMain(queue string, args ...interface{}) error {
	log.WithFields(logrus.Fields{"queue": queue, "args": args}).Info("lol not really")
	return nil
}
