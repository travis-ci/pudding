package main

import (
	"fmt"
	"os"

	"github.com/codegangsta/cli"
	"github.com/travis-pro/pudding/lib"
	"github.com/travis-pro/pudding/lib/workers"
)

func main() {
	app := cli.NewApp()
	app.Version = lib.VersionString
	app.Flags = []cli.Flag{
		lib.RedisURLFlag,
		cli.StringFlag{
			Name:   "redis-pool-size",
			Value:  "30",
			EnvVar: "PUDDING_REDIS_POOL_SIZE",
		},
		cli.StringFlag{
			Name:   "q, queues",
			Value:  "instance-builds,instance-terminations",
			EnvVar: "QUEUES",
		},
		cli.StringFlag{
			Name: "P, process-id",
			Value: func() string {
				v := os.Getenv("DYNO")
				if v == "" {
					v = fmt.Sprintf("%d", os.Getpid())
				}
				return v
			}(),
			EnvVar: "PUDDING_PROCESS_ID",
		},
		cli.StringFlag{
			Name:   "K, aws-key",
			EnvVar: "AWS_ACCESS_KEY_ID",
		},
		cli.StringFlag{
			Name:   "S, aws-secret",
			EnvVar: "AWS_SECRET_ACCESS_KEY",
		},
		cli.StringFlag{
			Name:   "R, aws-region",
			Value:  "us-east-1",
			EnvVar: "AWS_DEFAULT_REGION",
		},
		cli.StringFlag{
			Name: "instance-rsa",
		},
		cli.StringFlag{
			Name:   "H, web-hostname",
			Usage:  "publicly-accessible hostname with protocol",
			Value:  "http://localhost:42151",
			EnvVar: "PUDDING_WEB_HOSTNAME",
		},
		cli.StringFlag{
			Name: "Y, instance-yml",
		},
		cli.StringFlag{
			Name: "T, init-script-template",
		},
		cli.IntFlag{
			Name:   "I, mini-worker-interval",
			Value:  30,
			Usage:  "interval in seconds for the mini worker loop",
			EnvVar: "PUDDING_MINI_WORKER_INTERVAL",
		},
		lib.SlackTeamFlag,
		lib.SlackTokenFlag,
		lib.SentryDSNFlag,
		lib.InstanceExpiryFlag,
		lib.DebugFlag,
	}
	app.Action = runWorkers
	app.Run(os.Args)
}

func runWorkers(c *cli.Context) {
	instanceRSA := c.String("instance-rsa")
	if instanceRSA == "" {
		instanceRSA = lib.GetInstanceRSAKey()
	}

	instanceYML := c.String("instance-yml")
	if instanceYML == "" {
		instanceYML = lib.GetInstanceYML()
	}

	initScriptTemplate := c.String("init-script-template")
	if initScriptTemplate == "" {
		initScriptTemplate = lib.GetInitScriptTemplate()
	}

	lib.WriteFlagsToEnv(c)

	workers.Main(&workers.Config{
		ProcessID:   c.String("process-id"),
		WebHostname: c.String("web-hostname"),
		Debug:       c.Bool("debug"),

		Queues:        c.String("queues"),
		RedisPoolSize: c.String("redis-pool-size"),
		RedisURL:      c.String("redis-url"),

		AWSKey:    c.String("aws-key"),
		AWSSecret: c.String("aws-secret"),
		AWSRegion: c.String("aws-region"),

		InstanceRSA:        instanceRSA,
		InstanceYML:        instanceYML,
		InitScriptTemplate: initScriptTemplate,
		MiniWorkerInterval: c.Int("mini-worker-interval"),
		InstanceExpiry:     c.Int("instance-expiry"),

		SlackTeam:  c.String("slack-team"),
		SlackToken: c.String("slack-token"),

		SentryDSN: c.String("sentry-dsn"),
	})
}
