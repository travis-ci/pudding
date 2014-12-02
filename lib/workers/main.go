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
func Main(cfg *Config) {
	if cfg.Debug {
		log.Level = logrus.DebugLevel
	}

	ic := &internalConfig{
		RedisPoolSize: cfg.RedisPoolSize,

		SlackTeam:  cfg.SlackTeam,
		SlackToken: cfg.SlackToken,

		SentryDSN: cfg.SentryDSN,

		WebHost:   cfg.WebHostname,
		ProcessID: cfg.ProcessID,

		InstanceRSA: cfg.InstanceRSA,
		InstanceYML: cfg.InstanceYML,

		Queues:             []string{},
		QueueConcurrencies: map[string]int{},
		QueueFuncs:         defaultQueueFuncs,

		MiniWorkerInterval:  cfg.MiniWorkerInterval,
		InstanceStoreExpiry: cfg.InstanceExpiry,
		ImageStoreExpiry:    cfg.ImageExpiry,

		InitScriptTemplate: template.Must(template.New("init-script").Parse(cfg.InitScriptTemplate)),
	}

	auth, err := aws.GetAuth(cfg.AWSKey, cfg.AWSSecret)
	if err != nil {
		log.WithField("err", err).Fatal("failed to load aws auth")
		os.Exit(1)
	}

	region, ok := aws.Regions[cfg.AWSRegion]
	if !ok {
		log.WithField("region", cfg.AWSRegion).Fatal("invalid region")
		os.Exit(1)
	}
	ic.AWSAuth = auth
	ic.AWSRegion = region

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
