package lib

import (
	"io/ioutil"
	"os"
	"path/filepath"
)

// GetDockerRSAKey attempts to retrieve the docker rsa private key
// from compressed env vars DOCKER_RSA and
// PUDDING_DOCKER_RSA, then falls back to attempting to read
// $PWD/docker_rsa
func GetDockerRSAKey() string {
	value := getDockerRSAKeyFromEnv()
	if value != "" {
		return value
	}

	return getDockerRSAKeyFromFile()
}

func getDockerRSAKeyFromEnv() string {
	for _, key := range []string{"DOCKER_RSA", "PUDDING_DOCKER_RSA"} {
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
