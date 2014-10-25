package common

import "os"

func GetTravisWorkerYML() string {
	for _, key := range []string{"TRAVIS_WORKER_YML", "WORKER_MANAGER_TRAVIS_WORKER_YML"} {
		value, err := GetCompressedEnvVar(key)
		if err == nil {
			return value
		}
	}

	return os.Getenv("travis_config")
}
