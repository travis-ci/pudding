package lib

// InstanceBuildPayload is the InstanceBuild representation sent to
// the background workers
type InstanceBuildPayload struct {
	Args       []*InstanceBuild `json:"args"`
	Queue      string           `json:"queue,omitempty"`
	JID        string           `json:"jid,omitempty"`
	Retry      bool             `json:"retry,omitempty"`
	EnqueuedAt float64          `json:"enqueued_at,omitempty"`
}

// InstanceBuild returns the inner instance build from the Args
// slice
func (ibp *InstanceBuildPayload) InstanceBuild() *InstanceBuild {
	if len(ibp.Args) < 1 {
		return nil
	}

	return ibp.Args[0]
}
