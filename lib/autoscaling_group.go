package lib

// AutoscalingGroup is the internal representation of an EC2
// autoscaling group
type AutoscalingGroup struct {
	Name            string `json:"name" redis:"name"`
	InstanceID      string `json:"instance_id" redis:"instance_id"`
	Queue           string `json:"queue" redis:"queue"`
	Env             string `json:"env" redis:"env"`
	Site            string `json:"site" redis:"site"`
	Role            string `json:"role" redis:"role"`
	MinSize         int    `json:"min_size" redis:"min_size"`
	MaxSize         int    `json:"max_size" redis:"max_size"`
	DesiredCapacity int    `json:"desired_capacity" redis:"desired_capacity"`
}

// Hydrate is used to overwrite "null" defaults that result from
// serialize/deserialize via JSON
func (asg *AutoscalingGroup) Hydrate() {
	if asg.MinSize == 0 {
		asg.MinSize = 1
	}
	if asg.MaxSize == 0 {
		asg.MaxSize = 1
	}
	if asg.DesiredCapacity == 0 {
		asg.DesiredCapacity = 1
	}
}
