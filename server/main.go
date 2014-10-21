package server

import "log"

// Main is the whole shebang
func Main(addr, authToken, redisURL string, queueNames map[string]string) {
	srv, err := newServer(addr, authToken, redisURL, queueNames)
	if err != nil {
		log.Fatalf("BOOM: %q", err)
	}
	srv.Setup()
	srv.Run()
}
