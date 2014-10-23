package workers

import (
	"net/url"

	"github.com/jrallison/go-workers"
	"github.com/mitchellh/goamz/aws"
)

type config struct {
	AWSAuth   aws.Auth
	AWSRegion aws.Region

	RedisURL      *url.URL
	RedisPoolSize string

	DockerRSA string
	ProcessID string
	SetupRSA  string

	Queues             []string
	QueueFuncs         map[string]func(*config, *workers.Msg)
	QueueConcurrencies map[string]int
}
