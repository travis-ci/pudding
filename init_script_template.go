package pudding

// GetInitScriptTemplate attempts to get the init script template
// from the `INIT_SCRIPT_TEMPLATE` and
// `PUDDING_INIT_SCRIPT_TEMPLATE` compressed env vars
func GetInitScriptTemplate() string {
	for _, key := range []string{"INIT_SCRIPT_TEMPLATE", "PUDDING_INIT_SCRIPT_TEMPLATE"} {
		value, err := GetCompressedEnvVar(key)
		if err == nil {
			return value
		}
	}
	return ""
}
