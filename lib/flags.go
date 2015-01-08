package lib

import (
	"fmt"
	"os"
	"strings"

	"github.com/codegangsta/cli"
)

var (
	// AddrFlag is the flag used for the server address, checking
	// also for the presence of the PORT env var
	AddrFlag = cli.StringFlag{
		Name: "a, addr",
		Value: func() string {
			v := ":" + os.Getenv("PORT")
			if v == ":" {
				v = ":42151"
			}
			return v
		}(),
		EnvVar: "PUDDING_ADDR",
	}
	// RedisURLFlag is the flag used to specify the redis URL, and
	// checks for REDIS_PROVIDER and REDIS_URL before defaulting to a
	// local redis addr
	RedisURLFlag = cli.StringFlag{
		Name: "r, redis-url",
		Value: func() string {
			provider := os.Getenv("REDIS_PROVIDER")
			if provider != "" {
			}
			v := os.Getenv(provider)
			if v == "" {
				v = os.Getenv("REDIS_URL")
			}
			if v == "" {
				v = "redis://localhost:6379/0"
			}
			return v
		}(),
		EnvVar: "PUDDING_REDIS_URL",
	}
	// InstanceExpiryFlag is the flag used for defining the expiry
	// used in redis when storing instance metadata
	InstanceExpiryFlag = cli.IntFlag{
		Name:   "E, instance-expiry",
		Value:  90,
		Usage:  "expiry in seconds for instance attributes",
		EnvVar: "PUDDING_INSTANCE_EXPIRY",
	}
	// ImageExpiryFlag is the flag used for defining the expiry
	// used in redis when storing image metadata
	ImageExpiryFlag = cli.IntFlag{
		Name:   "image-expiry",
		Value:  90,
		Usage:  "expiry in seconds for image attributes",
		EnvVar: "PUDDING_IMAGE_EXPIRY",
	}
	// SlackHookPathFlag is the incoming webhook path for slack integration
	SlackHookPathFlag = cli.StringFlag{
		Name:   "slack-hook-path",
		EnvVar: "PUDDING_SLACK_HOOK_PATH",
	}
	// SlackUsernameFlag is the username for slack integration
	SlackUsernameFlag = cli.StringFlag{
		Name:   "slack-username",
		Value:  "puddingbot",
		EnvVar: "PUDDING_SLACK_USERNAME",
	}
	// SlackChannelFlag is the default channel used when no channel
	// is provided in a web request
	SlackChannelFlag = cli.StringFlag{
		Name:   "default-slack-channel",
		Usage:  "default slack channel to use if none provided with request",
		Value:  "#general",
		EnvVar: "PUDDING_DEFAULT_SLACK_CHANNEL",
	}
	// SlackIconFlag is the default channel used when no channel
	// is provided in a web request
	SlackIconFlag = cli.StringFlag{
		Name:   "slack-icon",
		Value:  ":travis:",
		EnvVar: "PUDDING_SLACK_ICON",
	}
	// SentryDSNFlag is the dsn string used to initialize raven
	// clients
	SentryDSNFlag = cli.StringFlag{
		Name:   "sentry-dsn",
		Value:  os.Getenv("SENTRY_DSN"),
		EnvVar: "PUDDING_SENTRY_DSN",
	}
	// DebugFlag enables debug logging
	DebugFlag = cli.BoolFlag{
		Name:   "debug",
		EnvVar: "DEBUG",
	}
)

// WriteFlagsToEnv takes the parsed *cli.Context and writes flag
// values back into the os env, mostly for purposes of exposing via
// the server `/debug/vars` route.
func WriteFlagsToEnv(c *cli.Context) {
	for _, fl := range c.App.Flags {
		switch flVal := fl.(type) {
		case cli.StringFlag:
			names := strings.Split(flVal.Name, ",")
			if len(names) < 1 {
				continue
			}

			v := c.String(names[0])
			envVar := flVal.EnvVar
			if v != "" && envVar != "" {
				os.Setenv(envVar, v)
			}
		case cli.IntFlag:
			names := strings.Split(flVal.Name, ",")
			if len(names) < 1 {
				continue
			}

			v := c.Int(names[0])
			envVar := flVal.EnvVar
			if envVar != "" {
				os.Setenv(envVar, fmt.Sprintf("%d", v))
			}
		case cli.BoolFlag:
			names := strings.Split(flVal.Name, ",")
			if len(names) < 1 {
				continue
			}

			v := c.Bool(names[0])
			envVar := flVal.EnvVar
			if envVar != "" {
				os.Setenv(envVar, fmt.Sprintf("%v", v))
			}
		}
	}
}
