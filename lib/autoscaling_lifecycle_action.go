package lib

// AutoscalingLifecycleAction is an SNS message payload of the form:
// {
//   "AutoScalingGroupName":"name string",
//   "Service":"prose goop string",
//   "Time":"iso 8601 timestamp string",
//   "AccountId":"account id string",
//   "LifecycleTransition":"transition string, e.g.: autoscaling:EC2_INSTANCE_TERMINATING",
//   "RequestId":"uuid string",
//   "LifecycleActionToken":"uuid string",
//   "EC2InstanceId":"instance id string",
//   "LifecycleHookName":"name string"
// }
type AutoscalingLifecycleAction struct {
	AutoScalingGroupName string
	Service              string
	Time                 string
	AccountID            string `json:"AccountId"`
	LifecycleTransition  string
	RequestID            string `json:"RequestId"`
	LifecycleActionToken string
	EC2InstanceID        string `json:"EC2InstanceId"`
	LifecycleHookName    string
}
