package workers

// Config is everything needed to run the workers
type Config struct {
	ProcessID   string
	WebHostname string

	Queues        string
	RedisPoolSize string
	RedisURL      string

	AWSKey    string
	AWSSecret string
	AWSRegion string

	DockerRSA          string
	WorkerYML          string
	InitScriptTemplate string
	MiniWorkerInterval int
	InstanceExpiry     int

	SlackTeam  string
	SlackToken string

	SentryDSN string
}
