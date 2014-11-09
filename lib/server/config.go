package server

// Config is everything needed to run the server
type Config struct {
	Addr      string
	AuthToken string
	RedisURL  string

	SlackToken          string
	SlackTeam           string
	DefaultSlackChannel string

	SentryDSN string

	InstanceExpiry int

	QueueNames map[string]string
}
