package pudding

// InstanceTerminationPayload is the representation used when
// enqueueing an instance termination to the background workers
type InstanceTerminationPayload struct {
	JID          string `json:"jid,omitempty"`
	Retry        bool   `json:"retry,omitempty"`
	InstanceID   string `json:"instance_id"`
	SlackChannel string `json:"slack_channel"`
}
