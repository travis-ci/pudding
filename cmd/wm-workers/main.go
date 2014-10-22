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
			Name:   "q, queues",
			Value:  "instance-builds",
			EnvVar: "QUEUES",
		},
	}
	app.Action = runWorkers
	app.Run(os.Args)
}

func runWorkers(c *cli.Context) {
	workers.Main(c.String("queues"), c.String("redis-url"))
}
