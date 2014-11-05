package workers

import (
	"net/url"
	"text/template"

	"github.com/jrallison/go-workers"
	"github.com/mitchellh/goamz/aws"
)

type config struct {
	AWSAuth   aws.Auth
	AWSRegion aws.Region

	RedisURL      *url.URL
	RedisPoolSize string

	SlackTeam  string
	SlackToken string

	SentryDSN string

	WebHost   string
	ProcessID string

	DockerRSA       string
	TravisWorkerYML string

	Queues             []string
	QueueFuncs         map[string]func(*config, *workers.Msg)
	QueueConcurrencies map[string]int

	MiniWorkerInterval  int
	InstanceStoreExpiry int

	InitScriptTemplate *template.Template
}