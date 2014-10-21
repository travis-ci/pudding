package server

type instanceBuildsCollectionSingular struct {
	InstanceBuilds *instanceBuild `json:"instance_builds"`
}

type instanceBuildsCollection struct {
	InstanceBuilds []*instanceBuild `json:"instance_builds"`
}

type instanceBuild struct {
	Site         string `json:"site"`
	Env          string `json:"env"`
	AMI          string `json:"ami"`
	InstanceType string `json:"instance_type"`
	Count        int    `json:"count"`
	Queue        string `json:"queue"`
	HREF         string `json:"href,omitempty"`
	ID           string `json:"id,omitempty"`
}

func (b *instanceBuild) Validate() []error {
	return []error{}
}

func (b *instanceBuild) UpdateFromDetails(d *instanceBuildDetails) {
	return
}
