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
	"github.com/meatballhat/expvarplus"
	"github.com/meatballhat/negroni-logrus"
	"github.com/phyber/negroni-gzip/gzip"
	"github.com/travis-ci/pudding/lib"
	"github.com/travis-ci/pudding/lib/db"
	"github.com/travis-ci/pudding/lib/server/jsonapi"
	"github.com/travis-ci/pudding/lib/server/negroniraven"
)

var (
	errMissingInstanceBuildID = fmt.Errorf("missing instance build id")
	errMissingInstanceID      = fmt.Errorf("missing instance id")
	errKaboom                 = fmt.Errorf("simulated kaboom ʕノ•ᴥ•ʔノ ︵ ┻━┻")
	// errNotImplemented         = fmt.Errorf("not implemented nope nope nope")
	errUnknownInstance = fmt.Errorf("unknown instance")
)

const (
	stateOutOfServiceMsg = "is out of service :arrow_down:"
	stateInServiceMsg    = "is in service :arrow_up:"
)

func init() {
	expvarplus.AddToEnvWhitelist("BUILDPACK_URL",
		"DEBUG",
		"DYNO",
		"GENERATED",
		"HOSTNAME",
		"PORT",
		"QUEUES",
		"REVISION",
		"VERSION",

		"PUDDING_DEFAULT_SLACK_CHANNEL",
		"PUDDING_INIT_SCRIPT_TEMPLATE",
		"PUDDING_INSTANCE_BUILDS_QUEUE_NAME",
		"PUDDING_INSTANCE_EXPIRY",
		"PUDDING_INSTANCE_RSA",
		"PUDDING_INSTANCE_TERMINATIONS_QUEUE_NAME",
		"PUDDING_INSTANCE_YML",
		"PUDDING_MINI_WORKER_INTERVAL",
		"PUDDING_PROCESS_ID",
		"PUDDING_REDIS_POOL_SIZE",
		"PUDDING_REDIS_URL",
		"PUDDING_SENTRY_DSN",
		"PUDDING_SLACK_TEAM",
		"PUDDING_TEMPORARY_INIT_EXPIRY",
		"PUDDING_WEB_HOSTNAME")
}

type server struct {
	addr, authToken, slackHookPath, slackUsername, slackIcon, slackChannel, sentryDSN string

	log        *logrus.Logger
	builder    *instanceBuilder
	asgBuilder *autoscalingGroupBuilder
	snsHandler *snsHandler
	iltHandler *instanceLifecycleTransitionHandler
	terminator *instanceTerminator
	auther     *serverAuther
	is         db.InitScriptGetterAuther
	i          db.InstanceFetcherStorer
	img        db.ImageFetcherStorer

	n *negroni.Negroni
	r *mux.Router
	s *manners.GracefulServer
}

func newServer(cfg *Config) (*server, error) {
	log := logrus.New()
	if cfg.Debug {
		log.Level = logrus.DebugLevel
	}

	r, err := db.BuildRedisPool(cfg.RedisURL)
	if err != nil {
		return nil, err
	}

	builder, err := newInstanceBuilder(r, cfg.QueueNames["instance-builds"])
	if err != nil {
		return nil, err
	}

	asgBuilder, err := newAutoscalingGroupBuilder(r, cfg.QueueNames["autoscaling-group-builds"])
	if err != nil {
		return nil, err
	}

	snsHandler, err := newSNSHandler(r, cfg.QueueNames["sns-messages"])
	if err != nil {
		return nil, err
	}

	iltHandler, err := newInstanceLifecycleTransitionHandler(r, cfg.QueueNames["instance-lifecycle-transitions"])
	if err != nil {
		return nil, err
	}

	terminator, err := newInstanceTerminator(r, cfg.QueueNames["instance-terminations"])
	if err != nil {
		return nil, err
	}

	i, err := db.NewInstances(r, log, cfg.InstanceExpiry)
	if err != nil {
		return nil, err
	}

	img, err := db.NewImages(r, log, cfg.ImageExpiry)
	if err != nil {
		return nil, err
	}

	is, err := db.NewInitScripts(r, log)
	if err != nil {
		return nil, err
	}

	auther, err := newServerAuther(cfg.AuthToken, r, log)
	if err != nil {
		return nil, err
	}

	srv := &server{
		addr:      cfg.Addr,
		authToken: cfg.AuthToken,
		auther:    auther,

		slackHookPath: cfg.SlackHookPath,
		slackUsername: cfg.SlackUsername,
		slackIcon:     cfg.SlackIcon,
		slackChannel:  cfg.DefaultSlackChannel,

		sentryDSN: cfg.SentryDSN,

		builder:    builder,
		asgBuilder: asgBuilder,
		snsHandler: snsHandler,
		iltHandler: iltHandler,
		terminator: terminator,
		is:         is,
		i:          i,
		img:        img,
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
	_ = srv.s.ListenAndServe(srv.addr, srv.n)
}

func (srv *server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	srv.r.ServeHTTP(w, req)
}

func (srv *server) setupRoutes() {
	srv.r.HandleFunc(`/`, srv.handleGetRoot).Methods("GET").Name("ohai")
	srv.r.HandleFunc(`/`, srv.ifAuth(srv.handleDeleteRoot)).Methods("DELETE").Name("shutdown")
	srv.r.HandleFunc(`/debug/vars`, srv.ifAuth(expvarplus.HandleExpvars)).Methods("GET").Name("expvars")
	srv.r.HandleFunc(`/kaboom`, srv.ifAuth(srv.handleKaboom)).Methods("POST").Name("kaboom")

	srv.r.HandleFunc(`/autoscaling-group-builds`, srv.ifAuth(srv.handleAutoscalingGroupBuildsCreate)).Methods("POST").Name("autoscaling-group-builds-create")

	srv.r.HandleFunc(`/instances`, srv.ifAuth(srv.handleInstances)).Methods("GET").Name("instances")
	srv.r.HandleFunc(`/instances/{instance_id}`, srv.ifAuth(srv.handleInstanceByIDFetch)).Methods("GET").Name("instances-by-id")
	srv.r.HandleFunc(`/instances/{instance_id}`, srv.ifAuth(srv.handleInstanceByIDTerminate)).Methods("DELETE").Name("delete-instances-by-id")

	srv.r.HandleFunc(`/instance-builds`, srv.ifAuth(srv.handleInstanceBuildsCreate)).Methods("POST").Name("instance-builds-create")
	srv.r.HandleFunc(`/instance-builds/{uuid}`, srv.ifAuth(srv.handleInstanceBuildUpdateByID)).Methods("PATCH").Name("instance-builds-update-by-id")

	srv.r.HandleFunc(`/instance-launches/{uuid}`, srv.ifAuth(srv.handleInstanceLaunchesCreate)).Methods("POST").Name("instance-launches-create")

	srv.r.HandleFunc(`/instance-terminations/{uuid}`, srv.ifAuth(srv.handleInstanceTerminationsCreate)).Methods("POST").Name("instance-terminations-create")

	srv.r.HandleFunc(`/instance-heartbeats/{uuid}`, srv.ifAuth(srv.handleInstanceHeartbeat)).Methods("POST").Name("instance-heartbeats")

	srv.r.HandleFunc(`/init-scripts/{uuid}`, srv.ifAuth(srv.handleInitScripts)).Methods("GET").Name("init-scripts")

	srv.r.HandleFunc(`/sns-messages`, srv.handleSNSMessages).Name("sns-messages")

	srv.r.HandleFunc(`/images`, srv.ifAuth(srv.handleImages)).Methods("GET").Name("images")
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
	for _, qv := range []string{"env", "site", "role", "queue"} {
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
		srv.log.WithFields(logrus.Fields{
			"err":         err,
			"instance_id": vars["instance_id"],
		}).Error("failed to fetch instance")
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

func (srv *server) handleInstanceHeartbeat(w http.ResponseWriter, req *http.Request) {
	instanceID := req.FormValue("instance-id")
	if instanceID == "" {
		jsonapi.Error(w, errMissingInstanceID, http.StatusBadRequest)
		return
	}

	instances, err := srv.i.Fetch(map[string]string{"instance_id": instanceID})
	if err != nil {
		jsonapi.Error(w, err, http.StatusInternalServerError)
		return
	}

	if len(instances) < 1 {
		jsonapi.Error(w, errUnknownInstance, http.StatusNotFound)
		return
	}

	instance := instances[0]

	if instance.ExpectedState == "" {
		instance.ExpectedState = "up"
	}

	// XXX: the response format isn't really jsonapi, but I don't want the remote to have specific knowledge of
	// "instances" (grumble)
	jsonapi.Respond(w, instance, http.StatusOK)
}

func (srv *server) handleInstanceBuildUpdateByID(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	instanceBuildID, ok := vars["uuid"]
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

	slackChannel := req.FormValue("slack-channel")
	if slackChannel == "" {
		slackChannel = srv.slackChannel
	}

	instanceID := req.FormValue("instance-id")
	instances, err := srv.i.Fetch(map[string]string{"instance_id": instanceID})
	if err != nil {
		srv.log.WithFields(logrus.Fields{
			"err":         err,
			"instance_id": instanceID,
		}).Error("failed to fetch instance details")
		jsonapi.Error(w, errUnknownInstance, http.StatusNotFound)
		return
	}

	// FIXME: extract this bit for other notification types?
	if srv.slackHookPath != "" && slackChannel != "" && len(instances) > 0 {
		srv.log.Debug("sending slack notification!")
		inst := instances[0]

		notifier := lib.NewSlackNotifier(srv.slackHookPath, srv.slackUsername, srv.slackIcon)
		err := notifier.Notify(slackChannel,
			fmt.Sprintf("Finished starting instance `%s` for instance build *%s* %s",
				instanceID, instanceBuildID, lib.NotificationInstanceSummary(inst)))
		if err != nil {
			srv.log.WithField("err", err).Error("failed to send slack notification")
		}
	} else {
		srv.log.WithFields(logrus.Fields{
			"slack_hook_path": srv.slackHookPath,
		}).Debug("slack fields empty or no matching instances?")
	}

	jsonapi.Respond(w, map[string]string{"sure": "why not"}, http.StatusOK)
}

func (srv *server) handleAutoscalingGroupBuildsCreate(w http.ResponseWriter, req *http.Request) {
	payload := &lib.AutoscalingGroupBuildsCollectionSingular{}
	err := json.NewDecoder(req.Body).Decode(payload)
	if err != nil {
		jsonapi.Error(w, err, http.StatusBadRequest)
		return
	}

	build := payload.AutoscalingGroupBuilds
	if build.ID == "" {
		build.ID = feeds.NewUUID().String()
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

	build, err = srv.asgBuilder.Build(build)
	if err != nil {
		jsonapi.Error(w, err, http.StatusInternalServerError)
		return
	}

	jsonapi.Respond(w, &lib.AutoscalingGroupBuildsCollection{
		AutoscalingGroupBuilds: []*lib.AutoscalingGroupBuild{build},
	}, http.StatusAccepted)
}

func (srv *server) handleInstanceLaunchesCreate(w http.ResponseWriter, req *http.Request) {
	srv.handleInstanceLifecycleTransition("launching", w, req)
}

func (srv *server) handleInstanceTerminationsCreate(w http.ResponseWriter, req *http.Request) {
	srv.handleInstanceLifecycleTransition("terminating", w, req)
}

func (srv *server) handleInstanceLifecycleTransition(transition string, w http.ResponseWriter, req *http.Request) {
	t := &lib.InstanceLifecycleTransition{}

	err := json.NewDecoder(req.Body).Decode(t)
	if err != nil {
		jsonapi.Error(w, err, http.StatusBadRequest)
		return
	}

	t.Transition = transition
	t.ID = feeds.NewUUID().String()

	_, err = srv.iltHandler.Handle(t)
	if err != nil {
		jsonapi.Error(w, err, http.StatusInternalServerError)
		return
	}

	slackChannel := req.FormValue("slack-channel")
	if slackChannel == "" {
		slackChannel = srv.slackChannel
	}

	instances, _ := srv.i.Fetch(map[string]string{"instance_id": t.InstanceID})

	if srv.slackHookPath != "" && slackChannel != "" && instances != nil && len(instances) > 0 {
		srv.log.Debug("sending slack notification!")
		inst := instances[0]
		notifier := lib.NewSlackNotifier(srv.slackHookPath, srv.slackUsername, srv.slackIcon)
		stateMsg := ""
		switch transition {
		case "terminating":
			stateMsg = stateOutOfServiceMsg
		case "launching":
			stateMsg = stateInServiceMsg
		}
		if stateMsg != "" {
			err := notifier.Notify(slackChannel, fmt.Sprintf("Instance `%s` is %s %s",
				t.InstanceID, stateMsg, lib.NotificationInstanceSummary(inst)))
			if err != nil {
				srv.log.WithField("err", err).Error("failed to send slack notification")
			}
		}
	}

	jsonapi.Respond(w, map[string]string{"yay": t.InstanceID}, http.StatusOK)
}

func (srv *server) handleInitScripts(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	instanceBuildID, ok := vars["uuid"]
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

func (srv *server) handleSNSMessages(w http.ResponseWriter, req *http.Request) {
	msg := lib.NewSNSMessage()

	err := json.NewDecoder(req.Body).Decode(&msg)
	if err != nil {
		jsonapi.Error(w, err, http.StatusBadRequest)
		return
	}

	_, err = srv.snsHandler.Handle(msg)
	if err != nil {
		jsonapi.Error(w, err, http.StatusInternalServerError)
		return
	}

	jsonapi.Respond(w, map[string][]*lib.SNSMessage{
		"sns_messages": []*lib.SNSMessage{msg},
	}, http.StatusOK)
}

func (srv *server) handleImages(w http.ResponseWriter, req *http.Request) {
	f := map[string]string{}
	for _, qv := range []string{"active", "role"} {
		v := req.FormValue(qv)
		if v != "" {
			f[qv] = v
		}
	}

	images, err := srv.img.Fetch(f)
	if err != nil {
		jsonapi.Error(w, err, http.StatusInternalServerError)
		return
	}

	jsonapi.Respond(w, map[string][]*lib.Image{
		"images": images,
	}, http.StatusOK)
}
