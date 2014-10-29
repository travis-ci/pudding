package workers

import (
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/jrallison/go-workers"
	"github.com/mitchellh/goamz/aws"
)

var (
	log *logrus.Logger

	defaultQueueFuncs = map[string]func(*config, *workers.Msg){}
)

func init() {
	log = logrus.New()

	// FIXME: move this elsewhere
	if os.Getenv("DEBUG") != "" {
		log.Level = logrus.DebugLevel
	}
}

// Main is the whole shebang
func Main(queues, redisPoolSize, redisURLString, processID,
	awsKey, awsSecret, awsRegion, dockerRSA, webHost,
	papertrailSite, travisWorkerYML, slackTeam, slackToken string, miniWorkerInterval, instanceExpiry int) {

	cfg := &config{
		RedisPoolSize: redisPoolSize,

		WebHost:   webHost,
		ProcessID: processID,

		SlackTeam:  slackTeam,
		SlackToken: slackToken,

		DockerRSA:       dockerRSA,
		PapertrailSite:  papertrailSite,
		TravisWorkerYML: travisWorkerYML,

		Queues:              []string{},
		QueueConcurrencies:  map[string]int{},
		QueueFuncs:          defaultQueueFuncs,
		MiniWorkerInterval:  miniWorkerInterval,
		InstanceStoreExpiry: instanceExpiry,
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

	runWorkers(cfg, log)
}

func runWorkers(cfg *config, log *logrus.Logger) {
	// TODO: implement the raven middleware
	// workers.Middleware.Prepend(NewRavenMiddleware(sentryDSN))
	workers.Configure(optsFromConfig(cfg))

	for _, queue := range cfg.Queues {
		registered, ok := cfg.QueueFuncs[queue]
		if !ok {
			log.WithField("queue", queue).Warn("no worker func available for queue")
			continue
		}

		workers.Process(queue, func(msg *workers.Msg) {
			registered(cfg, msg)
		}, cfg.QueueConcurrencies[queue])
	}

	go setupMiniWorkers(cfg, log).Run()
	workers.Run()
}

func setupMiniWorkers(cfg *config, log *logrus.Logger) *miniWorkers {
	mw := newMiniWorkers(cfg, log)
	mw.Register("ec2-sync", func() error {
		syncer, err := newEC2Syncer(cfg, log)
		if err != nil {
			log.WithField("err", err).Error("failed to build syncer")
			return err
		}

		return syncer.Sync()
	})

	mw.Register("keepalive", func() error {
		_, err := http.Get(cfg.WebHost)
		if err != nil {
			log.WithField("err", err).Error("failed to hit web host")
			return err
		}

		return nil
	})

	return mw
}

func optsFromConfig(cfg *config) map[string]string {
	opts := map[string]string{
		"server":    cfg.RedisURL.Host,
		"database":  strings.TrimLeft(cfg.RedisURL.Path, "/"),
		"pool":      cfg.RedisPoolSize,
		"process":   cfg.ProcessID,
		"namespace": "worker-manager",
	}

	if cfg.RedisURL.User != nil {
		if p, ok := cfg.RedisURL.User.Password(); ok {
			opts["password"] = p
		}
	}

	return opts
}
