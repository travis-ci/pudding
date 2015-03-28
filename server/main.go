package server

import "log"

// Main is the whole shebang
func Main(cfg *Config) {
	srv, err := newServer(cfg)

	if err != nil {
		log.Fatalf("BOOM: %q", err)
	}
	srv.Setup()
	srv.Run()
}
