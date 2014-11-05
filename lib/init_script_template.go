package lib

// GetInitScriptTemplate attempts to get the init script template
// from the `INIT_SCRIPT_TEMPLATE` and
// `WORKER_MANAGER_INIT_SCRIPT_TEMPLATE` compressed env vars
func GetInitScriptTemplate() string {
	for _, key := range []string{"INIT_SCRIPT_TEMPLATE", "WORKER_MANAGER_INIT_SCRIPT_TEMPLATE"} {
		value, err := GetCompressedEnvVar(key)
		if err == nil {
			return value
		}
	}
	return ""
}
