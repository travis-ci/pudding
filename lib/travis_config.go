package lib

import (
	"fmt"
	"os"

	"github.com/hamfist/yaml"
)

var (
	errMissingSiteConfig = fmt.Errorf("missing \"site\" sub-config")
	errMissingEnvConfig  = fmt.Errorf("missing \"env\" sub-config")
)

// MultiEnvSiteTravisYML represents a travis config yml structure
// that generally has two levels of nesting below the
// concern-specific keys, one for site and another for env.
type MultiEnvSiteTravisYML struct {
	AMQP       map[string]map[string]*amqpConfig  `yaml:"amqp"`
	Build      map[string]map[string]*buildConfig `yaml:"build"`
	Librato    map[string]*libratoConfig          `yaml:"librato"`
	Cache      map[string]map[string]*cacheConfig `yaml:"cache"`
	Papertrail map[string]string                  `yaml:"papertrail"`
}

type amqpConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Vhost    string `yaml:"vhost"`
	TLS      string `yaml:"tls,omitempty"`
}

type buildConfig struct {
	APIToken string `yaml:"api_token"`
	URL      string `yaml:"url"`
}

type libratoConfig struct {
	Email string `yaml:"email"`
	Token string `yaml:"token"`
}

type cacheConfig struct {
	Type         string    `yaml:"type"`
	S3           *s3config `yaml:"s3"`
	FetchTimeout int       `yaml:"fetch_timeout"`
	PushTimeout  int       `yaml:"push_timeout"`
}

type s3config struct {
	AccessKeyID     string `yaml:"access_key_id"`
	SecretAccessKey string `yaml:"secret_access_key"`
	Bucket          string `yaml:"bucket"`
}

// WorkerTravisYML is the worker-specific configuration generated
// from a MultiEnvSiteTravisYML
type WorkerTravisYML struct {
	Env            string           `yaml:"env"`
	LinuxConfig    *workerEnvConfig `yaml:"linux,omitempty"`
	PapertrailSite string           `yaml:"papertrail_site,omitempty"`
}

func (wty *WorkerTravisYML) String() (string, error) {
	out, err := yaml.Marshal(wty)
	if out == nil {
		out = []byte{}
	}
	return string(out), err
}

type workerEnvConfig struct {
	Host              string            `yaml:"host"`
	LogLevel          string            `yaml:"log_level"`
	Queue             string            `yaml:"queue"`
	AMQP              *amqpConfig       `yaml:"amqp"`
	VMs               *vmsConfig        `yaml:"vms"`
	Build             *buildConfig      `yaml:"build"`
	Docker            *dockerConfig     `yaml:"docker"`
	Paranoid          bool              `yaml:"paranoid"`
	SkipResolvUpdates bool              `yaml:"skip_resolv_updates"`
	SkipEtcHostsFix   bool              `yaml:"skip_etc_hosts_fix"`
	Librato           *libratoConfig    `yaml:"librato"`
	LanguageMappings  map[string]string `yaml:"language_mappings"`
	CacheOptions      *cacheConfig      `yaml:"cache_options"`
	Timeouts          *timeoutsConfig   `yaml:"timeouts"`
}

type vmsConfig struct {
	Provider string `yaml:"provider"`
	Count    int    `yaml:"count"`
}

type dockerConfig struct {
	PrivateKeyPath string `yaml:"private_key_path"`
}

type timeoutsConfig struct {
	HardLimit int `yaml:"hard_limit"`
}

// BuildTravisWorkerYML accepts a string form of
// MultiEnvSiteTravisYML, site, env, queue, and count, and
// constructs a worker-specific configuration
func BuildTravisWorkerYML(site, env, rawYML, queue string, count int) (*WorkerTravisYML, error) {
	multiYML := &MultiEnvSiteTravisYML{
		AMQP:       map[string]map[string]*amqpConfig{},
		Build:      map[string]map[string]*buildConfig{},
		Librato:    map[string]*libratoConfig{},
		Cache:      map[string]map[string]*cacheConfig{},
		Papertrail: map[string]string{},
	}

	err := yaml.Unmarshal([]byte(rawYML), multiYML)
	if err != nil {
		return nil, err
	}

	amqpSite, ok := multiYML.AMQP[site]
	if !ok {
		return nil, errMissingSiteConfig
	}

	amqp, ok := amqpSite[env]
	if !ok {
		return nil, errMissingEnvConfig
	}

	buildSite, ok := multiYML.Build[site]
	if !ok {
		return nil, errMissingSiteConfig
	}

	build, ok := buildSite[env]
	if !ok {
		return nil, errMissingEnvConfig
	}

	librato, ok := multiYML.Librato[site]
	if !ok {
		return nil, errMissingSiteConfig
	}

	cacheSite, ok := multiYML.Cache[site]
	if !ok {
		return nil, errMissingSiteConfig
	}

	cache, ok := cacheSite[env]
	if !ok {
		return nil, errMissingEnvConfig
	}

	ps, ok := multiYML.Papertrail[site]
	if !ok {
		return nil, errMissingSiteConfig
	}

	wty := &WorkerTravisYML{
		Env: "linux",
		LinuxConfig: &workerEnvConfig{
			Host:     "$TRAVIS_WORKER_HOST_NAME",
			LogLevel: "info",
			Queue:    fmt.Sprintf("builds.%s", queue),
			AMQP:     amqp,
			VMs: &vmsConfig{
				Provider: "docker",
				Count:    count,
			},
			Build: build,
			Docker: &dockerConfig{
				PrivateKeyPath: "/home/deploy/.ssh/docker_rsa",
			},
			Paranoid:          true,
			SkipResolvUpdates: true,
			SkipEtcHostsFix:   true,
			Librato:           librato,
			LanguageMappings: map[string]string{
				"clojure": "jvm",
				"scala":   "jvm",
				"groovy":  "jvm",
				"java":    "jvm",
			},
			CacheOptions: cache,
			Timeouts: &timeoutsConfig{
				HardLimit: 7200,
			},
		},
		PapertrailSite: ps,
	}

	return wty, err
}

// GetTravisWorkerYML attempts to look up the MultiEnvSiteTravisYML
// string as a compressed env var at both TRAVIS_WORKER_YML and
// WORKER_MANAGER_TRAVIS_WORKER_YML, then falls back to a lookup
// of an uncompressed travis_config var.
func GetTravisWorkerYML() string {
	for _, key := range []string{"TRAVIS_WORKER_YML", "WORKER_MANAGER_TRAVIS_WORKER_YML"} {
		value, err := GetCompressedEnvVar(key)
		if err == nil {
			return value
		}
	}

	return os.Getenv("travis_config")
}
