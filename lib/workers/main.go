package workers

import (
	"net/url"
	"os"
	"strconv"
	"strings"
	"text/template"

	"github.com/Sirupsen/logrus"
	"github.com/mitchellh/goamz/aws"
)

// Main is the whole shebang
func Main(queues, redisPoolSize, redisURLString, processID,
	awsKey, awsSecret, awsRegion, dockerRSA, webHost,
	travisWorkerYML, slackTeam, slackToken, sentryDSN,
	initScriptTemplateString string,
	miniWorkerInterval, instanceExpiry int) {

	cfg := &config{
		RedisPoolSize: redisPoolSize,

		WebHost:   webHost,
		ProcessID: processID,

		SlackTeam:  slackTeam,
		SlackToken: slackToken,

		SentryDSN: sentryDSN,

		DockerRSA:       dockerRSA,
		TravisWorkerYML: travisWorkerYML,

		Queues:             []string{},
		QueueConcurrencies: map[string]int{},
		QueueFuncs:         defaultQueueFuncs,

		MiniWorkerInterval:  miniWorkerInterval,
		InstanceStoreExpiry: instanceExpiry,

		InitScriptTemplate: template.Must(template.New("init-script").Parse(initScriptTemplateString)),
	}

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

	if cfg.DockerRSA == "" {
		log.Fatal("missing docker rsa key")
		os.Exit(1)
	}

	for _, queue := range strings.Split(queues, ",") {
		concurrency := 10
		qParts := strings.Split(queue, ":")
		if len(qParts) == 2 {
			queue = qParts[0]
			parsedConcurrency, err := strconv.ParseUint(qParts[1], 10, 64)
			if err != nil {
				log.WithFields(logrus.Fields{
					"err":   err,
					"queue": queue,
				}).Warn("failed to parse concurrency for queue, defaulting to 10")
				concurrency = 10
			} else {
				concurrency = int(parsedConcurrency)
			}
		}
		queue = strings.TrimSpace(queue)
		cfg.QueueConcurrencies[queue] = concurrency
		cfg.Queues = append(cfg.Queues, queue)
	}

	redisURL, err := url.Parse(redisURLString)
	if err != nil {
		log.WithField("err", err).Fatal("failed to parse redis url")
		os.Exit(1)
	}

	cfg.RedisURL = redisURL

	err = runWorkers(cfg, log)
	if err != nil {
		log.WithField("err", err).Fatal("failed to start workers")
		os.Exit(1)
	}
}
