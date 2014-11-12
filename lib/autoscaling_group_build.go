package lib

import "strings"

// AutoscalingGroupBuildsCollectionSingular is the singular representation
// used in jsonapi bodies
type AutoscalingGroupBuildsCollectionSingular struct {
	AutoscalingGroupBuilds *AutoscalingGroupBuild `json:"autoscaling_group_builds"`
}

// AutoscalingGroupBuildsCollection is the collection representation used
// in jsonapi bodies
type AutoscalingGroupBuildsCollection struct {
	AutoscalingGroupBuilds []*AutoscalingGroupBuild `json:"autoscaling_group_builds"`
}

// AutoscalingGroupBuild contains everything needed by a background worker
// to build the autoscaling group
type AutoscalingGroupBuild struct {
	ID              string `json:"id,omitempty"`
	Name            string `json:"name" redis:"name"`
	InstanceID      string `json:"instance_id,omitempty"`
	NameTemplate    string `json:"name_template,omitempty"`
	Queue           string `json:"queue" redis:"queue"`
	Env             string `json:"env" redis:"env"`
	Site            string `json:"site" redis:"site"`
	Role            string `json:"role" redis:"role"`
	MinSize         int    `json:"min_size" redis:"min_size"`
	MaxSize         int    `json:"max_size" redis:"max_size"`
	DesiredCapacity int    `json:"desired_capacity" redis:"desired_capacity"`
	SlackChannel    string `json:"slack_channel"`
}

func NewAutoscalingGroupBuild() *AutoscalingGroupBuild {
	return &AutoscalingGroupBuild{}
}

// Hydrate is used to overwrite "null" defaults that result from
// serialize/deserialize via JSON
func (asgb *AutoscalingGroupBuild) Hydrate() {
	if asgb.MinSize == 0 {
		asgb.MinSize = 1
	}
	if asgb.MaxSize == 0 {
		asgb.MaxSize = 1
	}
	if asgb.DesiredCapacity == 0 {
		asgb.DesiredCapacity = 1
	}

	if asgb.NameTemplate == "" {
		asgb.NameTemplate = "{{.Role}}-{{.Site}}-{{.Env}}-{{.Queue}}-{{.InstanceIDWithoutPrefix}}"
	}
}

func (asgb *AutoscalingGroupBuild) InstanceIDWithoutPrefix() string {
	return strings.TrimPrefix(asgb.InstanceID, "i-")
}
