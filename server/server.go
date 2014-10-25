package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/braintree/manners"
	"github.com/codegangsta/negroni"
	"github.com/gorilla/feeds"
	"github.com/gorilla/mux"
	"github.com/meatballhat/negroni-logrus"
	"github.com/travis-pro/worker-manager-service/common"
	"github.com/travis-pro/worker-manager-service/server/jsonapi"
)

type server struct {
	addr      string
	authToken string

	log     *logrus.Logger
	builder *instanceBuilder

	n *negroni.Negroni
	r *mux.Router
	s *manners.GracefulServer
}

func newServer(addr, authToken, redisURL string, queueNames map[string]string) (*server, error) {
	builder, err := newInstanceBuilder(redisURL, queueNames["instance-builds"])
	if err != nil {
		return nil, err
	}
	srv := &server{
		addr:      addr,
		authToken: authToken,

		log:     logrus.New(),
		builder: builder,

		n: negroni.New(),
		r: mux.NewRouter(),
		s: manners.NewServer(),
	}

	return srv, nil
}

func (srv *server) Setup() {
	srv.setupRoutes()
	srv.setupMiddleware()
}

func (srv *server) Run() {
	srv.log.WithField("addr", srv.addr).Info("Listening")
	srv.s.ListenAndServe(srv.addr, srv.n)
}

func (srv *server) setupRoutes() {
	srv.r.HandleFunc(`/`,
		srv.handleRoot).Methods("GET", "DELETE").Name("root")
	srv.r.HandleFunc(`/instance-builds`,
		srv.handleInstanceBuilds).Methods("GET", "POST").Name("instance-builds")
	srv.r.HandleFunc(`/instances/{instance_id}/links/metadata`,
		srv.handleInstanceMetadata).Methods("GET", "PATCH", "PUT").Name("instance-links-metadata")
	srv.r.HandleFunc(`/instance-builds/{instance_build_id}/links/cloud-inits`,
		srv.handleInstanceBuildsCloudInits).Methods("POST", "GET").Name("instance-builds-links-cloud-inits")
}

func (srv *server) setupMiddleware() {
	srv.n.Use(newTokenAuthMiddleware(srv.authToken))
	srv.n.Use(negroni.NewRecovery())
	srv.n.Use(negronilogrus.NewMiddleware())
	// TODO: implement the raven middleware, eh
	// srv.n.Use(negroniraven.NewMiddleware(sentryDSN))
	srv.n.UseHandler(srv.r)
}

func (srv *server) handleRoot(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case "DELETE":
		w.WriteHeader(http.StatusNoContent)
		srv.s.Shutdown <- true
	case "GET":
		w.Header().Set("Content-Type", "text-plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "ohai\n")
	}
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
	payload := &common.InstanceBuildsCollectionSingular{}
	err := json.NewDecoder(req.Body).Decode(payload)
	if err != nil {
		jsonapi.Error(w, err, http.StatusBadRequest)
		return
	}

	build := payload.InstanceBuilds
	if build.ID == "" {
		build.ID = feeds.NewUUID().String()
	}

	validationErrors := build.Validate()
	if len(validationErrors) > 0 {
		jsonapi.Errors(w, validationErrors, http.StatusBadRequest)
		return
	}

	build, err = srv.builder.Build(build)
	if err != nil {
		jsonapi.Error(w, err, http.StatusInternalServerError)
		return
	}

	jsonapi.Respond(w, &common.InstanceBuildsCollection{
		InstanceBuilds: []*common.InstanceBuild{build},
	}, http.StatusAccepted)
}

func (srv *server) handleInstanceBuildsList(w http.ResponseWriter, req *http.Request) {
	jsonapi.Respond(w,
		&common.InstanceBuildsCollection{InstanceBuilds: []*common.InstanceBuild{}},
		http.StatusOK)
}

func (srv *server) handleInstanceMetadata(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case "GET":
		srv.handleInstanceMetadataGet(w, req)
	case "PATCH", "PUT":
		srv.handleInstanceMetadataUpdate(w, req)
	}
}

func (srv *server) handleInstanceMetadataGet(w http.ResponseWriter, req *http.Request) {
	http.Error(w, "not yet", http.StatusNotImplemented)
}

func (srv *server) handleInstanceMetadataUpdate(w http.ResponseWriter, req *http.Request) {
	http.Error(w, "not yet", http.StatusNotImplemented)
}

func (srv *server) handleInstanceBuildsCloudInits(w http.ResponseWriter, req *http.Request) {
	http.Error(w, "not yet", http.StatusNotImplemented)
}
