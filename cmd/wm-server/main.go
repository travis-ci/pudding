package main

import (
	"os"

	"github.com/codegangsta/cli"
	"github.com/travis-pro/worker-manager-service/lib"
	"github.com/travis-pro/worker-manager-service/lib/server"
)

func main() {
	app := cli.NewApp()
	app.Version = lib.VersionString
	app.Flags = []cli.Flag{
		lib.AddrFlag,
		lib.RedisURLFlag,
		cli.StringFlag{
			Name:   "instance-builds-queue-name",
			Value:  "instance-builds",
			EnvVar: "WORKER_MANAGER_INSTANCE_BUILDS_QUEUE_NAME",
		},
		cli.StringFlag{
			Name:   "instance-terminations-queue-name",
			Value:  "instance-terminations",
			EnvVar: "WORKER_MANAGER_INSTANCE_TERMINATIONS_QUEUE_NAME",
		},
		cli.StringFlag{
			Name:   "A, auth-token",
			Value:  "swordfish",
			EnvVar: "WORKER_MANAGER_AUTH_TOKEN",
		},
		lib.SlackTeamFlag,
		lib.SlackTokenFlag,
		lib.SlackChannelFlag,
		lib.SentryDSNFlag,
		lib.InstanceExpiryFlag,
	}
	app.Action = runServer

	app.Run(os.Args)
}

func runServer(c *cli.Context) {
	lib.WriteFlagsToEnv(c)

	server.Main(c.String("addr"), c.String("auth-token"), c.String("redis-url"),
		c.String("slack-token"), c.String("slack-team"), c.String("default-slack-channel"),
		c.String("sentry-dsn"),
		c.Int("instance-expiry"),
		map[string]string{
			"instance-builds":       c.String("instance-builds-queue-name"),
			"instance-terminations": c.String("instance-terminations-queue-name"),
		})
}
