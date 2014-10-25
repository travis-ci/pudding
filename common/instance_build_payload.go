package common

type InstanceBuildPayload struct {
	Class      string           `json:"class"`
	Args       []*InstanceBuild `json:"args"`
	Queue      string           `json:"queue,omitempty"`
	JID        string           `json:"jid,omitempty"`
	Retry      bool             `json:"retry,omitempty"`
	EnqueuedAt float64          `json:"enqueued_at,omitempty"`
}

func (ibp *InstanceBuildPayload) InstanceBuild() *InstanceBuild {
	if len(ibp.Args) < 1 {
		return nil
	}

	return ibp.Args[0]
}
