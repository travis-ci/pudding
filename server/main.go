package server

// Main is the whole shebang
func Main(addr, redisURL string) {
	srv := newServer(addr, redisURL)
	srv.Setup()
	srv.Run()
}
