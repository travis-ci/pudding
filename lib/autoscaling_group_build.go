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
	InstanceID string `json:"instance_id,omitempty"`
	ID         string `json:"id,omitempty"`
}
