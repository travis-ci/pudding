package pudding

// AutoscalingLifecycleAction is an SNS message payload of the form:
// {
//   "AutoScalingGroupName":"name string",
//   "Service":"prose goop string",
//   "Time":"iso 8601 timestamp string",
//   "AccountID":"account id string",
//   "LifecycleTransition":"transition string, e.g.: autoscaling:EC2_INSTANCE_TERMINATING",
//   "RequestID":"uuid string",
//   "LifecycleActionToken":"uuid string",
//   "EC2InstanceID":"instance id string",
//   "LifecycleHookName":"name string"
// }
type AutoscalingLifecycleAction struct {
	Event                string
	AutoScalingGroupName string `redis:"auto_scaling_group_name"`
	Service              string
	Time                 string
	AccountID            string `json:"AccountID"`
	LifecycleTransition  string
	RequestID            string `json:"RequestID"`
	LifecycleActionToken string `redis:"lifecycle_action_token"`
	EC2InstanceID        string `json:"EC2InstanceID"`
	LifecycleHookName    string `redis:"lifecycle_hook_name"`
}
