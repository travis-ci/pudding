package main

import (
	"os"

	"github.com/codegangsta/cli"
	"github.com/travis-pro/worker-manager-service/server"
)

func main() {
	app := cli.NewApp()
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name: "a, addr",
			Value: func() string {
				v := ":" + os.Getenv("PORT")
				if v == ":" {
					v = ":42151"
				}
				return v
			}(),
			EnvVar: "WORKER_MANAGER_ADDR",
		},
		cli.StringFlag{
			Name: "r, redis-url",
			Value: func() string {
				v := os.Getenv("REDISGREEN_URL")
				if v == "" {
					v = os.Getenv("REDIS_URL")
				}
				if v == "" {
					v = "redis://localhost:6379/0"
				}
				return v
			}(),
			EnvVar: "WORKER_MANAGER_REDIS_URL",
		},
	}
	app.Action = runServer

	app.Run(os.Args)
}

func runServer(c *cli.Context) {
	server.Main(c.String("addr"), c.String("redis-url"))
}
