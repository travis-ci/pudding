package common

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func GetDefaultRSAKey() string {
	allKeys, err := filepath.Glob(filepath.Join(os.Getenv("HOME"), ".ssh", "id_*"))
	if err != nil || allKeys == nil {
		return ""
	}

	sort.Strings(allKeys)

	for _, key := range allKeys {
		if strings.HasSuffix(".pub", key) {
			continue
		}

		b, err := ioutil.ReadFile(key)
		if err != nil {
			return ""
		}

		return string(b)
	}

	return ""
}

func GetDockerRSAKey() string {
	value := os.Getenv("DOCKER_RSA")
	if value != "" {
		return value
	}

	wd, err := os.Getwd()
	if err != nil {
		return ""
	}

	b, err := ioutil.ReadFile(filepath.Join(wd, "docker_rsa"))
	if err != nil {
		return ""
	}

	return string(b)
}
