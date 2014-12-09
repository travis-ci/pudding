package server

// Config is everything needed to run the server
type Config struct {
	Addr      string
	AuthToken string
	Debug     bool

	RedisURL string

	SlackToken          string
	SlackTeam           string
	SlackUsername       string
	DefaultSlackChannel string

	SentryDSN string

	InstanceExpiry int
	ImageExpiry    int

	QueueNames map[string]string
}
