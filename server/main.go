package server

func Main(addr, redisURL string) {
	srv := newServer(addr, redisURL)
	srv.Setup()
	srv.Run()
}
