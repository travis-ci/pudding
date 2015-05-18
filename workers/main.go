package workers

import (
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/awslabs/aws-sdk-go/aws"
)

// Main is the whole shebang
func Main(cfg *Config) {
	if cfg.Debug {
		log.Level = logrus.DebugLevel
	}

	ic := &internalConfig{
		AWSConfig: aws.DefaultConfig,

		RedisPoolSize: cfg.RedisPoolSize,

		SlackHookPath: cfg.SlackHookPath,
		SlackUsername: cfg.SlackUsername,
		SlackIcon:     cfg.SlackIcon,

		SentryDSN: cfg.SentryDSN,

		WebHost:   cfg.WebHostname,
		ProcessID: cfg.ProcessID,

		InstanceRSA:        cfg.InstanceRSA,
		InstanceYML:        cfg.InstanceYML,
		InstanceTagRetries: cfg.InstanceTagRetries,

		Queues:             []string{},
		QueueConcurrencies: map[string]int{},
		QueueFuncs:         defaultQueueFuncs,

		MiniWorkerInterval:  cfg.MiniWorkerInterval,
		InstanceStoreExpiry: cfg.InstanceExpiry,
		ImageStoreExpiry:    cfg.ImageExpiry,

		InitScriptTemplateString: cfg.InitScriptTemplate,
	}

	ic.AWSConfig.Region = cfg.AWSRegion

	if ic.InstanceRSA == "" {
		log.Fatal("missing instance rsa key")
		os.Exit(1)
	}

	for _, queue := range strings.Split(cfg.Queues, ",") {
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
		ic.QueueConcurrencies[queue] = concurrency
		ic.Queues = append(ic.Queues, queue)
	}

	redisURL, err := url.Parse(cfg.RedisURL)
	if err != nil {
		log.WithField("err", err).Fatal("failed to parse redis url")
		os.Exit(1)
	}

	ic.RedisURL = redisURL

	err = runWorkers(ic, log)
	if err != nil {
		log.WithField("err", err).Fatal("failed to start workers")
		os.Exit(1)
	}
}
