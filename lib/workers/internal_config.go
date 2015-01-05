package workers

import (
	"net/url"
	"text/template"

	"github.com/jrallison/go-workers"
	"github.com/mitchellh/goamz/aws"
)

type internalConfig struct {
	AWSAuth   aws.Auth
	AWSRegion aws.Region

	RedisURL      *url.URL
	RedisPoolSize string

	SlackHookPath string
	SlackUsername string
	SlackIcon     string

	SentryDSN string

	WebHost   string
	ProcessID string

	InstanceRSA        string
	InstanceYML        string
	InstanceTagRetries int

	Queues             []string
	QueueFuncs         map[string]func(*internalConfig, *workers.Msg)
	QueueConcurrencies map[string]int

	MiniWorkerInterval  int
	InstanceStoreExpiry int
	ImageStoreExpiry    int
	CloudInitExpiry     int

	InitScriptTemplate *template.Template
}
