package common

import "fmt"

func InitScriptRedisKey(instanceBuildID string) string {
	return fmt.Sprintf("worker-manager:init-script:%s", instanceBuildID)
}

func AuthRedisKey(instanceBuildID string) string {
	return fmt.Sprintf("worker-manager:auth:%s", instanceBuildID)
}
