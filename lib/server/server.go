package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/braintree/manners"
	"github.com/codegangsta/negroni"
	"github.com/gorilla/feeds"
	"github.com/gorilla/mux"
	"github.com/meatballhat/expvarplus"
	"github.com/meatballhat/logrus"
	"github.com/meatballhat/negroni-logrus"
	"github.com/phyber/negroni-gzip/gzip"
	"github.com/travis-pro/worker-manager-service/lib"
	"github.com/travis-pro/worker-manager-service/lib/db"
	"github.com/travis-pro/worker-manager-service/lib/server/jsonapi"
	"github.com/travis-pro/worker-manager-service/lib/server/negroniraven"
)

var (
	errMissingInstanceBuildID = fmt.Errorf("missing instance build id")
	errMissingInstanceID      = fmt.Errorf("missing instance id")
	errKaboom                 = fmt.Errorf("simulated kaboom ʕノ•ᴥ•ʔノ ︵ ┻━┻")
)

func init() {
	expvarplus.AddToEnvWhitelist("VERSION", "REVISION", "GENERATED", "DYNO",
		"BUILDPACK_URL", "PORT", "DEBUG", "HOSTNAME", "WORKER_MANAGER_WEB_HOSTNAME",
		"WORKER_MANAGER_INSTANCE_EXPIRY", "WORKER_MANAGER_DEFAULT_SLACK_CHANNEL",
		"WORKER_MANAGER_SLACK_TEAM")
}

type server struct {
	addr, authToken, slackToken, slackTeam, slackChannel, sentryDSN string

	log        *logrus.Logger
	builder    *instanceBuilder
	terminator *instanceTerminator
	auther     *serverAuther
	is         db.InitScriptGetterAuther
	i          db.InstanceFetcherStorer

	n *negroni.Negroni
	r *mux.Router
	s *manners.GracefulServer
}

func newServer(addr, authToken, redisURL, slackToken, slackTeam, slackChannel, sentryDSN string,
	instanceExpiry int, queueNames map[string]string) (*server, error) {

	log := logrus.New()
	// FIXME: move this elsewhere
	if os.Getenv("DEBUG") != "" {
		log.Level = logrus.DebugLevel
	}

	builder, err := newInstanceBuilder(redisURL, queueNames["instance-builds"])
	if err != nil {
		return nil, err
	}

	terminator, err := newInstanceTerminator(redisURL, queueNames["instance-terminations"])
	if err != nil {
		return nil, err
	}

	i, err := db.NewInstances(redisURL, log, instanceExpiry)
	if err != nil {
		return nil, err
	}

	is, err := db.NewInitScripts(redisURL, log)
	if err != nil {
		return nil, err
	}

	auther, err := newServerAuther(authToken, redisURL, log)
	if err != nil {
		return nil, err
	}

	srv := &server{
		addr:      addr,
		authToken: authToken,
		auther:    auther,

		slackToken:   slackToken,
		slackTeam:    slackTeam,
		slackChannel: slackChannel,

		sentryDSN: sentryDSN,

		builder:    builder,
		terminator: terminator,
		is:         is,
		i:          i,
		log:        log,

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
	srv.r.HandleFunc(`/`, srv.handleGetRoot).Methods("GET").Name("ohai")
	srv.r.HandleFunc(`/`, srv.ifAuth(srv.handleDeleteRoot)).Methods("DELETE").Name("shutdown")
	srv.r.HandleFunc(`/debug/vars`, srv.ifAuth(expvarplus.HandleExpvars)).Methods("GET").Name("expvars")
	srv.r.HandleFunc(`/kaboom`, srv.ifAuth(srv.handleKaboom)).Methods("POST").Name("kaboom")
	srv.r.HandleFunc(`/instances`, srv.ifAuth(srv.handleInstances)).Methods("GET").Name("instances")
	srv.r.HandleFunc(`/instances/{instance_id}`, srv.ifAuth(srv.handleInstanceByIDFetch)).Methods("GET").Name("instances-by-id")
	srv.r.HandleFunc(`/instances/{instance_id}`, srv.ifAuth(srv.handleInstanceByIDTerminate)).Methods("DELETE").Name("delete-instances-by-id")
	srv.r.HandleFunc(`/instance-builds`, srv.ifAuth(srv.handleInstanceBuildsCreate)).Methods("POST").Name("instance-builds-create")
	srv.r.HandleFunc(`/instance-builds/{instance_build_id}`, srv.ifAuth(srv.handleInstanceBuildUpdateByID)).Methods("PATCH").Name("instance-builds-update-by-id")
	srv.r.HandleFunc(`/init-scripts/{instance_build_id}`, srv.ifAuth(srv.handleInitScripts)).Methods("GET").Name("init-scripts")
}

func (srv *server) setupMiddleware() {
	srv.n.Use(negroni.NewRecovery())
	srv.n.Use(negronilogrus.NewMiddleware())
	srv.n.Use(gzip.Gzip(gzip.DefaultCompression))
	nr, err := negroniraven.NewMiddleware(srv.sentryDSN)
	if err != nil {
		panic(err)
	}
	srv.n.Use(nr)
	srv.n.UseHandler(srv.r)
}

func (srv *server) ifAuth(f func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		if !srv.auther.Authenticate(w, req) {
			return
		}

		f(w, req)
	}
}
func (srv *server) handleGetRoot(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text-plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "ohai\n")
}

func (srv *server) handleDeleteRoot(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusNoContent)
	srv.s.Shutdown <- true
}

func (srv *server) handleKaboom(w http.ResponseWriter, req *http.Request) {
	panic(errKaboom)
}

func (srv *server) handleInstances(w http.ResponseWriter, req *http.Request) {
	f := map[string]string{}
	for _, qv := range []string{"env", "site"} {
		v := req.FormValue(qv)
		if v != "" {
			f[qv] = v
		}
	}

	instances, err := srv.i.Fetch(f)
	if err != nil {
		jsonapi.Error(w, err, http.StatusInternalServerError)
		return
	}

	jsonapi.Respond(w, map[string][]*lib.Instance{
		"instances": instances,
	}, http.StatusOK)
}

func (srv *server) handleInstanceByIDFetch(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	instances, err := srv.i.Fetch(map[string]string{"instance_id": vars["instance_id"]})
	if err != nil {
		jsonapi.Error(w, err, http.StatusInternalServerError)
		return
	}

	jsonapi.Respond(w, map[string][]*lib.Instance{
		"instances": instances,
	}, http.StatusOK)
}

func (srv *server) handleInstanceByIDTerminate(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	instanceID, ok := vars["instance_id"]
	if !ok {
		jsonapi.Error(w, errMissingInstanceID, http.StatusBadRequest)
		return
	}

	err := srv.terminator.Terminate(instanceID, req.FormValue("slack-channel"))
	if err != nil {
		jsonapi.Error(w, err, http.StatusInternalServerError)
		return
	}

	jsonapi.Respond(w, map[string]string{"ok": "working on that"}, http.StatusAccepted)
}

func (srv *server) handleInstanceBuildsCreate(w http.ResponseWriter, req *http.Request) {
	payload := &lib.InstanceBuildsCollectionSingular{}
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

	if v := req.FormValue("slack-channel"); v != "" {
		build.SlackChannel = v
	}

	if build.SlackChannel == "" {
		build.SlackChannel = srv.slackChannel
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

	jsonapi.Respond(w, &lib.InstanceBuildsCollection{
		InstanceBuilds: []*lib.InstanceBuild{build},
	}, http.StatusAccepted)
}

func (srv *server) handleInstanceBuildUpdateByID(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	instanceBuildID, ok := vars["instance_build_id"]
	if !ok {
		jsonapi.Error(w, errMissingInstanceBuildID, http.StatusBadRequest)
		return
	}

	state := req.FormValue("state")
	if state != "finished" {
		srv.log.WithField("state", state).Debug("no-op state")
		jsonapi.Respond(w, map[string]string{"no": "op"}, http.StatusOK)
		return
	}

	instanceID := req.FormValue("instance-id")
	if instanceID == "" {
		instanceID = "?wat?"
	}

	slackChannel := req.FormValue("slack-channel")
	if slackChannel == "" {
		slackChannel = srv.slackChannel
	}

	if srv.slackTeam != "" && srv.slackToken != "" {
		srv.log.Debug("sending slack notification!")
		notifier := lib.NewSlackNotifier(srv.slackTeam, srv.slackToken)
		err := notifier.Notify(slackChannel,
			fmt.Sprintf("Finished starting instance `%s` for instance build *%s*", instanceID, instanceBuildID))
		if err != nil {
			srv.log.WithField("err", err).Error("failed to send slack notification")
		}
	} else {
		srv.log.WithFields(logrus.Fields{
			"slack_team":  srv.slackTeam,
			"slack_token": srv.slackToken,
		}).Debug("slack fields empty?")
	}

	err := srv.builder.Wipe(instanceBuildID)
	if err != nil {
		jsonapi.Error(w, err, http.StatusInternalServerError)
		return
	}

	jsonapi.Respond(w, map[string]string{"sure": "why not"}, http.StatusOK)
}

func (srv *server) handleInitScripts(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	instanceBuildID, ok := vars["instance_build_id"]
	if !ok {
		jsonapi.Error(w, errMissingInstanceBuildID, http.StatusBadRequest)
		return
	}

	srv.sendInitScript(w, instanceBuildID)
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
