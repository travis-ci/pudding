package common

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"io/ioutil"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/garyburd/redigo/redis"
)

type InstanceBuildAuther interface {
	HasValidAuth(string, string) bool
}

type InitScripts struct {
	RedisNamespace string
	redisURLString string
	r              *redis.Pool
}

func NewInitScripts(redisURL string) (*InitScripts, error) {
	is := &InitScripts{
		redisURLString: redisURL,
		RedisNamespace: RedisNamespace,
	}

	err := is.Setup()
	if err != nil {
		return nil, err
	}

	return is, nil
}

func (is *InitScripts) Setup() error {
	pool, err := BuildRedisPool(is.redisURLString)
	if err != nil {
		return err
	}

	is.r = pool
	return nil
}

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

func (is *InitScripts) HasValidAuth(ID, auth string) bool {
	conn := is.r.Get()
	defer conn.Close()

	dbAuth, err := redis.String(conn.Do("GET", AuthRedisKey(ID)))
	if err != nil {
		return false
	}

	log := logrus.New()
	log.WithFields(logrus.Fields{
		"instance_build_id": ID,
		"auth":              auth,
		"db_auth":           dbAuth,
	}).Info("comparing auths")

	return strings.TrimSpace(dbAuth) == strings.TrimSpace(auth)
}
