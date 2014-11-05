package server

import (
	"encoding/base64"
	"net/http"
	"regexp"
	"strings"

	"github.com/gorilla/feeds"
	"github.com/gorilla/mux"
	"github.com/meatballhat/logrus"
	"github.com/travis-pro/worker-manager-service/lib/db"
)

const (
	internalAuthHeader = "Worker-Manager-Internal-Is-Authorized"
)

var (
	basicAuthValueRegexp = regexp.MustCompile("(?i:^basic[= ])")
	instanceBuildRegexp  = regexp.MustCompile("(?:instance-builds|init-scripts)/(.*)")
)

type serverAuther struct {
	Token    string
	redisURL string
	is       db.InstanceBuildAuther
	log      *logrus.Logger
	rt       string
}

func newServerAuther(token, redisURL string, log *logrus.Logger) (*serverAuther, error) {
	sa := &serverAuther{
		Token:    token,
		redisURL: redisURL,
		log:      log,
		rt:       feeds.NewUUID().String(),
	}

	is, err := db.NewInitScripts(redisURL, log)
	if err != nil {
		return nil, err
	}

	sa.is = is
	return sa, nil
}

func (sa *serverAuther) Authenticate(w http.ResponseWriter, req *http.Request) bool {
	vars := mux.Vars(req)

	sa.log.WithFields(logrus.Fields{
		"path": req.URL.Path,
		"vars": vars,
	}).Debug("extracting instance build id if present")

	instanceBuildID, ok := vars["instance_build_id"]
	if !ok {
		matches := instanceBuildRegexp.FindStringSubmatch(req.URL.Path)
		if len(matches) > 1 {
			instanceBuildID = matches[1]
		}
	}

	authHeader := req.Header.Get("Authorization")
	sa.log.WithField("authorization", authHeader).Debug("raw authorization header")

	if authHeader != "" && (sa.hasValidTokenAuth(authHeader) || sa.hasValidInstanceBuildBasicAuth(authHeader, instanceBuildID)) {
		req.Header.Set(internalAuthHeader, sa.rt)
		sa.log.WithFields(logrus.Fields{
			"request_id":        req.Header.Get("X-Request-ID"),
			"instance_build_id": instanceBuildID,
		}).Debug("allowing authorized request yey")
		return true
	}

	if authHeader == "" {
		w.Header().Set("WWW-Authenticate", "token")
		sa.log.WithFields(logrus.Fields{
			"request_id": req.Header.Get("X-Request-ID"),
		}).Debug("responding 401 due to empty Authorization header")
		http.Error(w, "NO", http.StatusUnauthorized)
		return false
	}

	http.Error(w, "NO", http.StatusForbidden)
	return false
}

func (sa *serverAuther) hasValidTokenAuth(authHeader string) bool {
	if authHeader == ("token "+sa.Token) || authHeader == ("token="+sa.Token) {
		sa.log.Debug("taken auth matches yey")
		return true
	}

	sa.log.Debug("token auth does not match")
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
	}).Debug("checking basic auth against database")
	return sa.is.HasValidAuth(instanceBuildID, authParts[1])
}
