package server

import (
	"encoding/base64"
	"net/http"
	"regexp"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/feeds"
	"github.com/gorilla/mux"
	"github.com/travis-pro/worker-manager-service/common"
)

var (
	basicAuthValueRegexp = regexp.MustCompile("(?i:^basic[= ])")
	instanceBuildRegexp  = regexp.MustCompile("instance-builds/(.*)")
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
	vars := mux.Vars(req)
	instanceBuildID, ok := vars["instance_build_id"]
	if !ok {
		matches := instanceBuildRegexp.FindStringSubmatch(req.URL.Path)
		if len(matches) > 1 {
			instanceBuildID = matches[1]
		}
	}

	authHeader := req.Header.Get("Authorization")
	sa.log.WithField("authorization", authHeader).Info("raw authorization header")

	if authHeader != "" && (sa.hasValidTokenAuth(authHeader) || sa.hasValidInstanceBuildBasicAuth(authHeader, instanceBuildID)) {
		req.Header.Set("Worker-Manager-Internal-Is-Authorized", sa.rt)
		sa.log.WithFields(logrus.Fields{
			"request_id":        req.Header.Get("X-Request-ID"),
			"instance_build_id": instanceBuildID,
		}).Info("allowing authorized request yey")
		next(w, req)
		return
	}

	if authHeader == "" {
		w.Header().Set("WWW-Authenticate", "token")
		sa.log.WithFields(logrus.Fields{
			"request_id": req.Header.Get("X-Request-ID"),
		}).Info("responding 401 due to empty Authorization header")
		http.Error(w, "NO", http.StatusUnauthorized)
		return
	}

	http.Error(w, "NO", http.StatusForbidden)
}

func (sa *serverAuther) hasValidTokenAuth(authHeader string) bool {
	if authHeader == ("token "+sa.Token) || authHeader == ("token="+sa.Token) {
		sa.log.Info("taken auth matches yey")
		return true
	}

	sa.log.Info("token auth does not match")
	return false
}

func (sa *serverAuther) hasValidInstanceBuildBasicAuth(authHeader, instanceBuildID string) bool {
	if !basicAuthValueRegexp.MatchString(authHeader) {
		return false
	}

	b64Auth := basicAuthValueRegexp.ReplaceAllString(authHeader, "")
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

	sa.log.WithFields(logrus.Fields{
		"basic_auth":        authParts[1],
		"instance_build_id": instanceBuildID,
	}).Info("checking basic auth against database")
	return sa.is.HasValidAuth(instanceBuildID, authParts[1])
}
