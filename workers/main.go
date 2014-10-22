package workers

import (
	"fmt"
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/benmanns/goworker"
	"github.com/travis-pro/worker-manager-service/common"
)

var (
	log *logrus.Logger
)

func init() {
	log = logrus.New()
}

func Main(queues, redisURL string) {
	os.Args = []string{
		"wm-workers",
		"-uri", redisURL,
		"-namespace", fmt.Sprintf("%s:", common.RedisNamespace),
		"-queues", queues,
		"-use-number", "true",
	}

	log.WithField("args", os.Args).Info("Setting os.Args prior to goworker.Work")
	err := goworker.Work()
	if err != nil {
		log.Error(err)
	}
}
