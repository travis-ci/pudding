package workers

type initScriptContext struct {
	Env              string
	Site             string
	InstanceRSA      string
	SlackChannel     string
	PapertrailSite   string
	InstanceYML      string
	InstanceBuildID  string
	InstanceBuildURL string
}
