package lib

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
