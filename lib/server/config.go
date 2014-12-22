package server

// Config is everything needed to run the server
type Config struct {
	Addr      string
	AuthToken string
	Debug     bool

	RedisURL string

	SlackHookPath       string
	SlackUsername       string
	SlackIcon           string
	DefaultSlackChannel string

	SentryDSN string

	InstanceExpiry int
	ImageExpiry    int

	QueueNames map[string]string
}
