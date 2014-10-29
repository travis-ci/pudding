package server

import "log"

// Main is the whole shebang
func Main(addr, authToken, redisURL, slackToken, slackTeam, slackChannel string,
	instanceExpiry int, queueNames map[string]string) {

	srv, err := newServer(addr, authToken, redisURL,
		slackToken, slackTeam, slackChannel,
		instanceExpiry, queueNames)

	if err != nil {
		log.Fatalf("BOOM: %q", err)
	}
	srv.Setup()
	srv.Run()
}
