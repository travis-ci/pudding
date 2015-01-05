package lib

// AutoscalingGroupBuildPayload is the AutoscalingGroupBuild
// representation sent to the background workers
type AutoscalingGroupBuildPayload struct {
	Args       []*AutoscalingGroupBuild `json:"args"`
	Queue      string                   `json:"queue,omitempty"`
	JID        string                   `json:"jid,omitempty"`
	Retry      bool                     `json:"retry,omitempty"`
	EnqueuedAt float64                  `json:"enqueued_at,omitempty"`
}

// AutoscalingGroupBuild casts the first argument to an AutoscalingGroupBuild type
func (asgbp *AutoscalingGroupBuildPayload) AutoscalingGroupBuild() *AutoscalingGroupBuild {
	if len(asgbp.Args) < 1 {
		return nil
	}

	return asgbp.Args[0]
}
