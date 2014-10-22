package main

import (
	"fmt"
	"os"

	"github.com/codegangsta/cli"
	"github.com/travis-pro/worker-manager-service/common"
	"github.com/travis-pro/worker-manager-service/server"
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
		common.AddrFlag,
		common.RedisURLFlag,
		cli.StringFlag{
			Name:   "instance-builds-queue-name",
			Value:  "instance-builds",
			EnvVar: "WORKER_MANAGER_INSTANCE_BUILDS_QUEUE_NAME",
		},
		cli.StringFlag{
			Name:   "A, auth-token",
			Value:  "swordfish",
			EnvVar: "WORKER_MANAGER_AUTH_TOKEN",
		},
	}
	app.Action = runServer

	app.Run(os.Args)
}

func runServer(c *cli.Context) {
	server.Main(c.String("addr"), c.String("auth-token"), c.String("redis-url"),
		map[string]string{
			"instance-builds": c.String("instance-builds-queue-name"),
		})
}
