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
			Value:  "instance-builds",
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
			Name:   "docker-rsa",
			Value:  common.GetDockerRSAKey(),
			EnvVar: "WORKER_MANAGER_DOCKER_RSA",
		},
		cli.StringFlag{
			Name:   "setup-rsa",
			Value:  common.GetDefaultRSAKey(),
			EnvVar: "WORKER_MANAGER_SETUP_RSA",
		},
	}
	app.Action = runWorkers
	app.Run(os.Args)
}

func runWorkers(c *cli.Context) {
	workers.Main(c.String("queues"), c.String("redis-pool-size"),
		c.String("redis-url"), c.String("process-id"),
		c.String("aws-key"), c.String("aws-secret"), c.String("aws-region"),
		c.String("docker-rsa"), c.String("setup-rsa"))
}
