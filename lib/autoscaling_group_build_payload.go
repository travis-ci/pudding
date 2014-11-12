package lib

// AutoscalingGroupBuildPayload is the AutoscalingGroupBuild
// representation sent to the background workers
type AutoscalingGroupBuildPayload struct {
	Args []*AutoscalingGroupBuild
}

func (asgbp *AutoscalingGroupBuildPayload) AutoscalingGroupBuild() *AutoscalingGroupBuild {
	if len(asgbp.Args) < 1 {
		return nil
	}

	return asgbp.Args[0]
}
