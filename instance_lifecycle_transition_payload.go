package pudding

// InstanceLifecycleTransitionPayload is the background job payload for instance lifecycle transitions
type InstanceLifecycleTransitionPayload struct {
	Args       []*InstanceLifecycleTransition `json:"args"`
	Queue      string                         `json:"queue,omitempty"`
	JID        string                         `json:"jid,omitempty"`
	Retry      bool                           `json:"retry,omitempty"`
	EnqueuedAt float64                        `json:"enqueued_at,omitempty"`
}

// InstanceLifecycleTransition returns the inner *InstanceLifecycleTransition if available
func (iltp *InstanceLifecycleTransitionPayload) InstanceLifecycleTransition() *InstanceLifecycleTransition {
	if len(iltp.Args) < 1 {
		return nil
	}

	return iltp.Args[0]
}
