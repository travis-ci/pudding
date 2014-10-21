package server

import (
	"fmt"
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/negroni"
	"github.com/gorilla/mux"
	"github.com/meatballhat/negroni-logrus"
)

func Main(addr string) {
	srv := newServer(addr)
	srv.Setup()
	srv.Run()
}

type server struct {
	addr string
	log  *logrus.Logger
	n    *negroni.Negroni
	r    *mux.Router
}

func newServer(addr string) *server {
	return &server{
		addr: addr,
		log:  logrus.New(),
		n:    negroni.New(),
		r:    mux.NewRouter(),
	}
}

func (srv *server) Setup() {
	srv.setupRoutes()
	srv.setupMiddleware()
}

func (srv *server) Run() {
	srv.log.WithField("addr", srv.addr).Info("Listening")
	srv.n.Run(srv.addr)
}

func (srv *server) setupRoutes() {
	srv.r.HandleFunc(`/`, srv.handleRoot).Methods("GET").Name("root")
}

func (srv *server) setupMiddleware() {
	srv.n.Use(negroni.NewRecovery())
	srv.n.Use(negronilogrus.NewMiddleware())
	srv.n.UseHandler(srv.r)
}

func (srv *server) handleRoot(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text-plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "ohai\n")
}
