package lib

import (
	"io/ioutil"
	"os"
	"path/filepath"
)

// GetInstanceRSAKey attempts to retrieve the docker rsa private key
// from compressed env vars DOCKER_RSA and
// PUDDING_DOCKER_RSA, then falls back to attempting to read
// $PWD/docker_rsa
func GetInstanceRSAKey() string {
	value := getInstanceRSAKeyFromEnv()
	if value != "" {
		return value
	}

	return getInstanceRSAKeyFromFile()
}

func getInstanceRSAKeyFromEnv() string {
	for _, key := range []string{"DOCKER_RSA", "PUDDING_DOCKER_RSA"} {
		value, err := GetCompressedEnvVar(key)
		if err == nil {
			return value
		}
	}

	return ""
}

func getInstanceRSAKeyFromFile() string {
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
