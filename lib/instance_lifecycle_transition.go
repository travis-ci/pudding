package lib

// InstanceLifecycleTransition is an event received from instances when launching and terminating
type InstanceLifecycleTransition struct {
	ID         string `json:"id,omitempty"`
	InstanceID string `json:"instance_id"`
	Transition string `json:"transition"`
}
