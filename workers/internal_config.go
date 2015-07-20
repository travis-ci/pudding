package workers

import (
	"net/url"
	"text/template"

	"github.com/awslabs/aws-sdk-go/aws"
	"github.com/jrallison/go-workers"
)

type internalConfig struct {
	AWSConfig *aws.Config

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

	InitScriptTemplate       *template.Template
	InitScriptTemplateString string
}
