package workers

type initScriptContext struct {
	Env                  string
	Site                 string
	Queue                string
	Role                 string
	AMI                  string
	Count                int
	InstanceType         string
	InstanceRSA          string
	SlackChannel         string
	PapertrailSite       string
	InstanceYML          string
	InstanceBuildID      string
	InstanceBuildURL     string
	InstanceLaunchURL    string
	InstanceTerminateURL string
}
