package common

import (
	"fmt"

	"github.com/hamfist/yaml"
)

var (
	errMissingSiteConfig = fmt.Errorf("missing \"site\" sub-config")
	errMissingEnvConfig  = fmt.Errorf("missing \"env\" sub-config")
)

type MultiConfigYML struct {
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

type WorkerYML struct {
	Env         string           `yaml:"env"`
	LinuxConfig *workerEnvConfig `yaml:"linux,omitempty"`
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

func BuildTravisWorkerYML(site, env, rawYML, queue string, count int) (string, error) {
	multiCfg := &MultiConfigYML{
		AMQP:       map[string]map[string]*amqpConfig{},
		Build:      map[string]map[string]*buildConfig{},
		Librato:    map[string]*libratoConfig{},
		Cache:      map[string]map[string]*cacheConfig{},
		Papertrail: map[string]string{},
	}

	err := yaml.Unmarshal([]byte(rawYML), multiCfg)
	if err != nil {
		return "", err
	}

	amqpSite, ok := multiCfg.AMQP[site]
	if !ok {
		return "", errMissingSiteConfig
	}

	amqp, ok := amqpSite[env]
	if !ok {
		return "", errMissingEnvConfig
	}

	buildSite, ok := multiCfg.Build[site]
	if !ok {
		return "", errMissingSiteConfig
	}

	build, ok := buildSite[env]
	if !ok {
		return "", errMissingEnvConfig
	}

	librato, ok := multiCfg.Librato[site]
	if !ok {
		return "", errMissingSiteConfig
	}

	cacheSite, ok := multiCfg.Cache[site]
	if !ok {
		return "", errMissingSiteConfig
	}

	cache, ok := cacheSite[env]
	if !ok {
		return "", errMissingEnvConfig
	}

	wc := &WorkerYML{
		Env: "linux",
		LinuxConfig: &workerEnvConfig{
			// FIXME: the instance id is not known at cloud init script
			// render time.  How fix?
			// Host: fmt.Sprintf("worker-linux-docker-%s.%s.travis-ci.%s",
			// strings.Replace(instanceID, "i-", "", -1), env, site),
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
	}

	out, err := yaml.Marshal(wc)
	return string(out), err
}
