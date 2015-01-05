package lib

import (
	"strings"
	"time"
)

// AutoscalingGroupBuildsCollectionSingular is the singular representation
// used in jsonapi bodies
type AutoscalingGroupBuildsCollectionSingular struct {
	AutoscalingGroupBuilds *AutoscalingGroupBuild `json:"autoscaling_group_builds"`
}

// AutoscalingGroupBuildsCollection is the collection representation used
// in jsonapi bodies
type AutoscalingGroupBuildsCollection struct {
	AutoscalingGroupBuilds []*AutoscalingGroupBuild `json:"autoscaling_group_builds"`
}

// AutoscalingGroupBuild contains everything needed by a background worker
// to build the autoscaling group
type AutoscalingGroupBuild struct {
	ID              string `json:"id,omitempty"`
	Name            string `json:"name" redis:"name"`
	InstanceID      string `json:"instance_id,omitempty"`
	RoleARN         string `json:"role_arn,omitempty"`
	TopicARN        string `json:"topic_arn,omitempty"`
	NameTemplate    string `json:"name_template,omitempty"`
	Queue           string `json:"queue" redis:"queue"`
	Env             string `json:"env" redis:"env"`
	Site            string `json:"site" redis:"site"`
	Role            string `json:"role" redis:"role"`
	MinSize         int    `json:"min_size" redis:"min_size"`
	MaxSize         int    `json:"max_size" redis:"max_size"`
	DesiredCapacity int    `json:"desired_capacity" redis:"desired_capacity"`
	SlackChannel    string `json:"slack_channel"`
	Timestamp       int64  `json:"timestamp"`
}

// NewAutoscalingGroupBuild makes a new AutoscalingGroupBuild
func NewAutoscalingGroupBuild() *AutoscalingGroupBuild {
	return &AutoscalingGroupBuild{}
}

// Hydrate is used to overwrite "null" defaults that result from
// serialize/deserialize via JSON
func (b *AutoscalingGroupBuild) Hydrate() {
	if b.NameTemplate == "" {
		b.NameTemplate = "{{.Role}}-{{.Site}}-{{.Env}}-{{.Queue}}-{{.InstanceIDWithoutPrefix}}-{{.Timestamp}}"
	}

	if b.Timestamp == 0 {
		b.Timestamp = time.Now().UTC().Unix()
	}
}

// Validate performs multiple validity checks and returns a slice of all errors
// found
func (b *AutoscalingGroupBuild) Validate() []error {
	errors := []error{}
	if b.InstanceID == "" {
		errors = append(errors, errEmptyInstanceID)
	}
	if b.Site == "" {
		errors = append(errors, errEmptySite)
	}
	if b.Env == "" {
		errors = append(errors, errEmptyEnv)
	}
	if b.Queue == "" {
		errors = append(errors, errEmptyQueue)
	}
	if b.Role == "" {
		errors = append(errors, errEmptyRole)
	}
	if b.RoleARN == "" {
		errors = append(errors, errEmptyRoleARN)
	}
	if b.TopicARN == "" {
		errors = append(errors, errEmptyTopicARN)
	}

	return errors
}

// InstanceIDWithoutPrefix returns the instance id without the "i-"
func (b *AutoscalingGroupBuild) InstanceIDWithoutPrefix() string {
	return strings.TrimPrefix(b.InstanceID, "i-")
}
