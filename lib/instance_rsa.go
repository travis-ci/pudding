package lib

import (
	"io/ioutil"
	"os"
	"path/filepath"
)

// GetInstanceRSAKey attempts to retrieve the instance rsa private
// key from compressed env vars INSTANCE_RSA and
// PUDDING_INSTANCE_RSA, then falls back to attempting to read
// $PWD/instance_rsa
func GetInstanceRSAKey() string {
	value := getInstanceRSAKeyFromEnv()
	if value != "" {
		return value
	}

	return getInstanceRSAKeyFromFile()
}

func getInstanceRSAKeyFromEnv() string {
	for _, key := range []string{"INSTANCE_RSA", "PUDDING_INSTANCE_RSA"} {
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

	b, err := ioutil.ReadFile(filepath.Join(wd, "instance_rsa"))
	if err != nil {
		return ""
	}

	return string(b)
}
