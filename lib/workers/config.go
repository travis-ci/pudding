package workers

// Config is everything needed to run the workers
type Config struct {
	ProcessID   string
	WebHostname string
	Debug       bool

	Queues        string
	RedisPoolSize string
	RedisURL      string

	AWSKey    string
	AWSSecret string
	AWSRegion string

	InstanceRSA        string
	InstanceYML        string
	InstanceTagRetries int

	InitScriptTemplate string
	MiniWorkerInterval int
	InstanceExpiry     int
	ImageExpiry        int

	SlackHookPath string
	SlackUsername string
	SlackIcon     string

	SentryDSN string
}
