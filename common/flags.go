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
	InstanceExpiryFlag = cli.IntFlag{
		Name:   "E, instance-expiry",
		Value:  90,
		Usage:  "expiry in seconds for instance attributes",
		EnvVar: "WORKER_MANAGER_INSTANCE_EXPIRY",
	}
	SlackTokenFlag = cli.StringFlag{
		Name:   "slack-token",
		EnvVar: "WORKER_MANAGER_SLACK_TOKEN",
	}
	SlackTeamFlag = cli.StringFlag{
		Name:   "slack-team",
		EnvVar: "WORKER_MANAGER_SLACK_TEAM",
	}
	SlackChannelFlag = cli.StringFlag{
		Name:   "default-slack-channel",
		Usage:  "default slack channel to use if none provided with request",
		Value:  "#general",
		EnvVar: "WORKER_MANAGER_DEFAULT_SLACK_CHANNEL",
	}
)
