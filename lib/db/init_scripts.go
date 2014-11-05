package db

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"io/ioutil"
	"strings"

	"github.com/meatballhat/logrus"
	"github.com/garyburd/redigo/redis"
)

// InstanceBuildAuther is the interface used to authenticate
// against temporary auth creds for download of init scripts via
// cloud-init on the remote instance
type InstanceBuildAuther interface {
	HasValidAuth(string, string) bool
}

// InitScriptGetterAuther is the extension of InstanceBuildAuther
// that performs the fetching of the init script for cloud-init
type InitScriptGetterAuther interface {
	InstanceBuildAuther
	Get(string) (string, error)
}

// InitScripts represents the internal init scripts collection
type InitScripts struct {
	r   *redis.Pool
	log *logrus.Logger
}

// NewInitScripts creates a new *InitScripts
func NewInitScripts(redisURL string, log *logrus.Logger) (*InitScripts, error) {
	r, err := BuildRedisPool(redisURL)
	if err != nil {
		return nil, err
	}

	return &InitScripts{
		r:   r,
		log: log,
	}, nil
}

// Get retrieves a given init script by ID, which is expected to be
// a uuid, although it really doesn't matter ☃
func (is *InitScripts) Get(ID string) (string, error) {
	conn := is.r.Get()
	defer conn.Close()

	b64Script, err := redis.String(conn.Do("GET", InitScriptRedisKey(ID)))
	if err != nil {
		return "", err
	}

	b, err := base64.StdEncoding.DecodeString(string(b64Script))
	if err != nil {
		return "", err
	}

	zr, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	defer zr.Close()

	script, err := ioutil.ReadAll(zr)
	if err != nil {
		return "", err
	}

	return string(script), nil
}

// HasValidAuth checks the provided temporary auth creds against
// what is stored in redis for the given init script id
func (is *InitScripts) HasValidAuth(ID, auth string) bool {
	conn := is.r.Get()
	defer conn.Close()

	redisKey := AuthRedisKey(ID)
	dbAuth, err := redis.String(conn.Do("GET", redisKey))
	if err != nil {
		is.log.WithFields(logrus.Fields{
			"err": err,
			"key": redisKey,
		}).Error("failed to fetch auth from database")
		return false
	}

	is.log.WithFields(logrus.Fields{
		"instance_build_id": ID,
		"auth":              auth,
		"db_auth":           dbAuth,
	}).Debug("comparing auths")

	return strings.TrimSpace(dbAuth) == strings.TrimSpace(auth)
}
