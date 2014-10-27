package server

import (
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/feeds"
	"github.com/gorilla/mux"
	"github.com/travis-pro/worker-manager-service/common"
)

type serverAuther struct {
	Token    string
	redisURL string
	is       common.InstanceBuildAuther
	log      *logrus.Logger
	rt       string
}

func newServerAuther(token, redisURL string) (*serverAuther, error) {
	sa := &serverAuther{
		Token:    token,
		redisURL: redisURL,
		log:      logrus.New(),
		rt:       feeds.NewUUID().String(),
	}

	is, err := common.NewInitScripts(redisURL)
	if err != nil {
		return nil, err
	}

	sa.is = is
	return sa, nil
}

func (sa *serverAuther) IsAuthorized(req *http.Request) bool {
	return req.Header.Get("Worker-Manager-Internal-Is-Authorized") == sa.rt
}

func (sa *serverAuther) ServeHTTP(w http.ResponseWriter, req *http.Request, next http.HandlerFunc) {
	isAuthd := false

	vars := mux.Vars(req)
	instanceBuildID, ok := vars["instance_build_id"]
	if !ok {
		instanceBuildID = ""
	}
	authHeader := req.Header.Get("Authorization")
	if sa.hasValidTokenAuth(authHeader) || sa.hasValidInstanceBuildBasicAuth(authHeader, instanceBuildID) {
		req.Header.Set("Worker-Manager-Internal-Is-Authorized", sa.rt)
		isAuthd = true
	}

	if req.Header.Get("Authorization") == "" {
		w.Header().Set("WWW-Authenticate", "token")
		http.Error(w, "NO", http.StatusUnauthorized)
		return
	}

	if isAuthd {
		next(w, req)
		return
	}

	http.Error(w, "NO", http.StatusForbidden)
}

func (sa *serverAuther) hasValidTokenAuth(authHeader string) bool {
	return authHeader == ("token "+sa.Token) || authHeader == ("token="+sa.Token)
}

func (sa *serverAuther) hasValidInstanceBuildBasicAuth(authHeader, instanceBuildID string) bool {
	if authHeader == "" {
		return false
	}

	b64Auth := strings.Replace(authHeader, "Basic ", "", -1)
	decoded, err := base64.StdEncoding.DecodeString(b64Auth)
	if err != nil {
		sa.log.WithField("err", err).Error("failed to base64 decade basic auth header")
		return false
	}

	authParts := strings.Split(string(decoded), ":")
	if len(authParts) != 2 {
		sa.log.Error("basic auth does not contain two parts")
		return false
	}

	return sa.is.HasValidAuth(instanceBuildID, authParts[1])
}
