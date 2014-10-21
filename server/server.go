package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/negroni"
	"github.com/gorilla/mux"
	"github.com/meatballhat/negroni-logrus"
	"github.com/travis-pro/worker-manager-service/server/jsonapi"
)

type server struct {
	addr    string
	log     *logrus.Logger
	builder *instanceBuilder

	n *negroni.Negroni
	r *mux.Router
}

func newServer(addr, redisURL string) *server {
	return &server{
		addr:    addr,
		log:     logrus.New(),
		builder: newInstanceBuilder(redisURL),

		n: negroni.New(),
		r: mux.NewRouter(),
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
	srv.r.HandleFunc(`/instance-builds`, srv.handleInstanceBuilds).Methods("GET", "POST").Name("instance-builds")
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

func (srv *server) handleInstanceBuilds(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case "POST":
		srv.handleInstanceBuildsCreate(w, req)
	case "GET":
		srv.handleInstanceBuildsList(w, req)
	}
}

func (srv *server) handleInstanceBuildsCreate(w http.ResponseWriter, req *http.Request) {
	payload := &instanceBuildsCollectionSingular{}
	err := json.NewDecoder(req.Body).Decode(payload)
	if err != nil {
		jsonapi.Error(w, err, http.StatusBadRequest)
		return
	}
	build := payload.InstanceBuilds
	validationErrors := build.Validate()
	if len(validationErrors) > 0 {
		jsonapi.Errors(w, validationErrors, http.StatusBadRequest)
		return
	}

	details, err := srv.builder.Build(build)
	if err != nil {
		jsonapi.Error(w, err, http.StatusInternalServerError)
		return
	}

	build.UpdateFromDetails(details)
	jsonapi.Respond(w, &instanceBuildsCollection{
		InstanceBuilds: []*instanceBuild{build},
	}, http.StatusAccepted)
}

func (srv *server) handleInstanceBuildsList(w http.ResponseWriter, req *http.Request) {
	jsonapi.Respond(w,
		&instanceBuildsCollection{InstanceBuilds: []*instanceBuild{}},
		http.StatusOK)
}
