package common

import (
	"io/ioutil"
	"os"
	"path/filepath"
)

func GetDockerRSAKey() string {
	value := getDockerRSAKeyFromEnv()
	if value != "" {
		return value
	}

	return getDockerRSAKeyFromFile()
}

func getDockerRSAKeyFromEnv() string {
	for _, key := range []string{"DOCKER_RSA", "WORKER_MANAGER_DOCKER_RSA"} {
		value, err := GetCompressedEnvVar(key)
		if err == nil {
			return value
		}
	}

	return ""
}

func getDockerRSAKeyFromFile() string {
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
