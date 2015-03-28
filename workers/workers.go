package workers

import (
	"net/http"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/garyburd/redigo/redis"
	"github.com/jrallison/go-workers"
)

var (
	log = logrus.New()

	defaultQueueFuncs = map[string]func(*internalConfig, *workers.Msg){}
)

func runWorkers(cfg *internalConfig, log *logrus.Logger) error {
	workers.Logger = log
	workers.Configure(optsFromConfig(cfg))

	rm, err := NewMiddlewareRaven(cfg.SentryDSN)
	if err != nil {
		log.WithFields(logrus.Fields{
			"sentry_dsn": cfg.SentryDSN,
			"err":        err,
		}).Error("failed to build sentry middleware")
		return err
	}

	workers.Middleware.Prepend(rm)

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

	go setupMiniWorkers(cfg, workers.Config.Pool, log, rm).Run()

	log.Info("starting go-workers")
	workers.Run()
	return nil
}

func setupMiniWorkers(cfg *internalConfig, r *redis.Pool, log *logrus.Logger, rm *MiddlewareRaven) *miniWorkers {
	mw := newMiniWorkers(cfg, log, rm)
	mw.Register("ec2-sync", func() error {
		syncer, err := newEC2Syncer(cfg, r, log)
		if err != nil {
			log.WithField("err", err).Error("failed to build syncer")
			return err
		}

		return syncer.Sync()
	})

	mw.Register("keepalive", func() error {
		_, err := http.Get(cfg.WebHost)
		if err != nil {
			log.WithField("err", err).Panic("failed to hit web host")
		}

		return nil
	})

	return mw
}

func optsFromConfig(cfg *internalConfig) map[string]string {
	opts := map[string]string{
		"server":    cfg.RedisURL.Host,
		"database":  strings.TrimLeft(cfg.RedisURL.Path, "/"),
		"pool":      cfg.RedisPoolSize,
		"process":   cfg.ProcessID,
		"namespace": "pudding",
	}

	if cfg.RedisURL.User != nil {
		if p, ok := cfg.RedisURL.User.Password(); ok {
			opts["password"] = p
		}
	}

	return opts
}
