package main

import (
	"fmt"
	"os"

	"github.com/codegangsta/cli"
	"github.com/travis-pro/worker-manager-service/common"
	"github.com/travis-pro/worker-manager-service/workers"
)

var (
	VersionString   = "?"
	RevisionString  = "?"
	GeneratedString = "?"
)

func customVersionPrinter(c *cli.Context) {
	fmt.Printf("%v v=%v rev=%v d=%v\n", c.App.Name, VersionString, RevisionString, GeneratedString)
}

func main() {
	cli.VersionPrinter = customVersionPrinter

	app := cli.NewApp()
	app.Flags = []cli.Flag{
		common.RedisURLFlag,
		cli.StringFlag{
			Name:   "redis-pool-size",
			Value:  "30",
			EnvVar: "WORKER_MANAGER_REDIS_POOL_SIZE",
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
			EnvVar: "WORKER_MANAGER_PROCESS_ID",
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
			Name: "docker-rsa",
		},
		cli.StringFlag{
			Name:   "H, web-hostname",
			Usage:  "publicly-accessible hostname with protocol",
			Value:  "http://localhost:42151",
			EnvVar: "WORKER_MANAGER_WEB_HOSTNAME",
		},
		cli.StringFlag{
			Name:   "papertrail-site",
			Usage:  "papertrail syslog upstream",
			Value:  "logs.papertrailapp.com:9999",
			EnvVar: "WORKER_MANAGER_PAPERTRAIL_SITE",
		},
		cli.StringFlag{
			Name: "Y, travis-worker-yml",
		},
		cli.IntFlag{
			Name:   "I, mini-worker-interval",
			Value:  30,
			Usage:  "interval in seconds for the mini worker loop",
			EnvVar: "WORKER_MANAGER_MINI_WORKER_INTERVAL",
		},
		common.SlackTeamFlag,
		common.SlackTokenFlag,
		common.InstanceExpiryFlag,
	}
	app.Action = runWorkers
	app.Run(os.Args)
}

func runWorkers(c *cli.Context) {
	dockerRSA := c.String("docker-rsa")
	if dockerRSA == "" {
		dockerRSA = common.GetDockerRSAKey()
	}

	workerYML := c.String("travis-worker-yml")
	if workerYML == "" {
		workerYML = common.GetTravisWorkerYML()
	}

	workers.Main(c.String("queues"), c.String("redis-pool-size"),
		c.String("redis-url"), c.String("process-id"),
		c.String("aws-key"), c.String("aws-secret"), c.String("aws-region"),
		dockerRSA, c.String("web-hostname"),
		c.String("papertrail-site"), workerYML,
		c.String("slack-team"), c.String("slack-token"),
		c.Int("mini-worker-interval"), c.Int("instance-expiry"))
}
