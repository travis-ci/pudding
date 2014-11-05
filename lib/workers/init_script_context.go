package workers

type initScriptContext struct {
	Env              string
	Site             string
	DockerRSA        string
	SlackChannel     string
	PapertrailSite   string
	TravisWorkerYML  string
	InstanceBuildID  string
	InstanceBuildURL string
}
