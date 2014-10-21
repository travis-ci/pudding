package server

import "log"

// Main is the whole shebang
func Main(addr, redisURL string, queueNames map[string]string) {
	srv, err := newServer(addr, redisURL, queueNames)
	if err != nil {
		log.Fatalf("BOOM: %q", err)
	}
	srv.Setup()
	srv.Run()
}
