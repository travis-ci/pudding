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
	"github.com/phyber/negroni-gzip/gzip"
	"github.com/travis-pro/worker-manager-service/common"
	"github.com/travis-pro/worker-manager-service/server/jsonapi"
)

var (
	errMissingInstanceBuildID = fmt.Errorf("missing instance build id")
)

type server struct {
	addr      string
	authToken string

	log     *logrus.Logger
	builder *instanceBuilder
	auther  *serverAuther
	is      *common.InitScripts

	n *negroni.Negroni
	r *mux.Router
	s *manners.GracefulServer
}

func newServer(addr, authToken, redisURL string, queueNames map[string]string) (*server, error) {
	builder, err := newInstanceBuilder(redisURL, queueNames["instance-builds"])
	if err != nil {
		return nil, err
	}

	is, err := common.NewInitScripts(redisURL)
	if err != nil {
		return nil, err
	}

	auther, err := newServerAuther(authToken, redisURL)
	if err != nil {
		return nil, err
	}

	srv := &server{
		addr:      addr,
		authToken: authToken,
		auther:    auther,

		log:     logrus.New(),
		builder: builder,
		is:      is,

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
	srv.r.HandleFunc(`/instance-builds/{instance_build_id}`,
		srv.handleInstanceBuildsByID).Methods("PATCH").Name("instance-builds-by-id")
	srv.r.HandleFunc(`/init-scripts/{instance_build_id}`,
		srv.handleInitScripts).Methods("GET").Name("init-scripts")
	srv.r.HandleFunc(`/instances/{instance_id}/links/metadata`,
		srv.handleInstanceMetadata).Methods("GET", "PATCH", "PUT").Name("instance-links-metadata")
}

func (srv *server) setupMiddleware() {
	srv.n.Use(srv.auther)
	srv.n.Use(negroni.NewRecovery())
	srv.n.Use(negronilogrus.NewMiddleware())
	srv.n.Use(gzip.Gzip(gzip.DefaultCompression))
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
		return
	case "GET":
		srv.handleInstanceBuildsList(w, req)
		return
	}

	http.Error(w, "NO", http.StatusMethodNotAllowed)
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

	if build.State == "" {
		build.State = "pending"
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

func (srv *server) handleInstanceBuildsByID(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case "PATCH":
		srv.handleInstanceBuildUpdateByID(w, req)
		return
	}

	http.Error(w, "NO", http.StatusMethodNotAllowed)
}

func (srv *server) handleInstanceBuildUpdateByID(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	instanceBuildID, ok := vars["instance_build_id"]
	if !ok {
		jsonapi.Error(w, errMissingInstanceBuildID, http.StatusBadRequest)
		return
	}

	state := req.FormValue("state")
	if state == "finished" {
		err := srv.builder.Wipe(instanceBuildID)
		if err != nil {
			jsonapi.Error(w, err, http.StatusInternalServerError)

			return
		}

		jsonapi.Respond(w, map[string]string{"sure": "why not"}, http.StatusOK)
		return
	}

	jsonapi.Respond(w, map[string]string{"no": "op"}, http.StatusOK)
}

func (srv *server) handleInitScripts(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	instanceBuildID, ok := vars["instance_build_id"]
	if !ok {
		jsonapi.Error(w, errMissingInstanceBuildID, http.StatusBadRequest)
		return
	}

	if srv.auther.IsAuthorized(req) {
		srv.sendInitScript(w, instanceBuildID)
		return
	}

	http.Error(w, "NO", http.StatusForbidden)
}

func (srv *server) sendInitScript(w http.ResponseWriter, ID string) {
	script, err := srv.is.Get(ID)
	if err != nil {
		srv.log.WithFields(logrus.Fields{
			"err": err,
			"id":  ID,
		}).Error("failed to get init script")
		jsonapi.Error(w, err, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/x-shellscript; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, script)
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
	jsonapi.Respond(w, map[string]string{"whatever": "sure"}, http.StatusOK)
	// http.Error(w, "not yet", http.StatusNotImplemented)
}

func (srv *server) handleInstanceMetadataUpdate(w http.ResponseWriter, req *http.Request) {
	jsonapi.Respond(w, map[string]string{"whatever": "sure"}, http.StatusOK)
	// http.Error(w, "not yet", http.StatusNotImplemented)
}
