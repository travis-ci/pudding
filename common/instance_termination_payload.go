package common

type InstanceTerminationPayload struct {
	InstanceID   string `json:"instance_id"`
	SlackChannel string `json:"slack_channel"`
}
