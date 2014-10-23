package workers

import (
	"net/url"
	"os"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/jrallison/go-workers"
	"github.com/mitchellh/goamz/aws"
)

var (
	log *logrus.Logger
	cfg *config
)

func init() {
	log = logrus.New()
	cfg = &config{}
}

func Main(queues, redisURLString, awsKey, awsSecret, awsRegion string) {
	auth, err := aws.GetAuth(awsKey, awsSecret)
	if err != nil {
		log.WithField("err", err).Fatal("failed to load aws auth")
		os.Exit(1)
	}

	region, ok := aws.Regions[awsRegion]
	if !ok {
		log.WithField("region", awsRegion).Fatal("invalid region")
		os.Exit(1)
	}
	cfg.AWSAuth = auth
	cfg.AWSRegion = region

	redisURL, err := url.Parse(redisURLString)
	if err != nil {
		log.WithField("err", err).Fatal("failed to parse redis url")
		os.Exit(1)
	}

	runWorkers(queues, redisURL)
}

func runWorkers(queues string, redisURL *url.URL) {
	opts := map[string]string{
		"server":    redisURL.Host,
		"database":  strings.TrimLeft(redisURL.Path, "/"),
		"pool":      "30",
		"process":   "1",
		"namespace": "worker-manager",
	}
	if redisURL.User != nil {
		if p, ok := redisURL.User.Password(); ok {
			opts["password"] = p
		}
	}
	workers.Configure(opts)

	for _, queue := range strings.Split(queues, ",") {
		switch queue {
		case "instance-builds":
			workers.Process(queue, instanceBuildsMain, 10)
		}
	}
	workers.Run()
}
