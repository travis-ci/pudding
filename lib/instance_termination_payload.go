package lib

// InstanceTerminationPayload is the representation used when
// enqueueing an instance termination to the background workers
type InstanceTerminationPayload struct {
	InstanceID   string `json:"instance_id"`
	SlackChannel string `json:"slack_channel"`
}
