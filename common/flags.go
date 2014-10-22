package common

import (
	"os"

	"github.com/codegangsta/cli"
)

var (
	AddrFlag = cli.StringFlag{
		Name: "a, addr",
		Value: func() string {
			v := ":" + os.Getenv("PORT")
			if v == ":" {
				v = ":42151"
			}
			return v
		}(),
		EnvVar: "WORKER_MANAGER_ADDR",
	}
	RedisURLFlag = cli.StringFlag{
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
	}
)
