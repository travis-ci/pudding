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

	ScaleOutCooldown                 int     `json:"scale_out_cooldown,omitempty"`
	ScaleOutAdjustment               int     `json:"scale_out_adjustment,omitempty"`
	ScaleOutMetricName               string  `json:"scale_out_metric_name,omitempty"`
	ScaleOutMetricNamespace          string  `json:"scale_out_metric_namespace,omitempty"`
	ScaleOutMetricStatistic          string  `json:"scale_out_metric_statistic,omitempty"`
	ScaleOutMetricPeriod             int     `json:"scale_out_metric_period,omitempty"`
	ScaleOutMetricEvaluationPeriods  int     `json:"scale_out_metric_evaluation_periods,omitempty"`
	ScaleOutMetricThreshold          float64 `json:"scale_out_metric_threshold,omitempty"`
	ScaleOutMetricComparisonOperator string  `json:"scale_out_metric_comparison_operator,omitempty"`

	ScaleInCooldown                 int     `json:"scale_in_cooldown,omitempty"`
	ScaleInAdjustment               int     `json:"scale_in_adjustment,omitempty"`
	ScaleInMetricName               string  `json:"scale_in_metric_name,omitempty"`
	ScaleInMetricNamespace          string  `json:"scale_in_metric_namespace,omitempty"`
	ScaleInMetricStatistic          string  `json:"scale_in_metric_statistic,omitempty"`
	ScaleInMetricPeriod             int     `json:"scale_in_metric_period,omitempty"`
	ScaleInMetricEvaluationPeriods  int     `json:"scale_in_metric_evaluation_periods,omitempty"`
	ScaleInMetricThreshold          float64 `json:"scale_in_metric_threshold,omitempty"`
	ScaleInMetricComparisonOperator string  `json:"scale_in_metric_comparison_operator,omitempty"`
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

	if b.ScaleOutCooldown == 0 {
		b.ScaleOutCooldown = 300
	}

	if b.ScaleInCooldown == 0 {
		b.ScaleInCooldown = 300
	}

	if b.ScaleOutAdjustment == 0 {
		b.ScaleOutAdjustment = 1
	}

	if b.ScaleInAdjustment == 0 {
		b.ScaleInAdjustment = -1
	}

	if b.ScaleOutMetricName == "" {
		b.ScaleOutMetricName = "CPUUtilization"
	}

	if b.ScaleOutMetricNamespace == "" {
		b.ScaleOutMetricNamespace = "AWS/EC2"
	}

	if b.ScaleOutMetricStatistic == "" {
		b.ScaleOutMetricStatistic = "Average"
	}

	if b.ScaleOutMetricPeriod == 0 {
		b.ScaleOutMetricPeriod = 120
	}

	if b.ScaleOutMetricEvaluationPeriods == 0 {
		b.ScaleOutMetricEvaluationPeriods = 2
	}

	if b.ScaleOutMetricThreshold == float64(0) {
		b.ScaleOutMetricThreshold = float64(90)
	}

	if b.ScaleOutMetricComparisonOperator == "" {
		b.ScaleOutMetricComparisonOperator = "GreaterThanOrEqualToThreshold"
	}

	if b.ScaleInMetricName == "" {
		b.ScaleInMetricName = "CPUUtilization"
	}

	if b.ScaleInMetricNamespace == "" {
		b.ScaleInMetricNamespace = "AWS/EC2"
	}

	if b.ScaleInMetricStatistic == "" {
		b.ScaleInMetricStatistic = "Average"
	}

	if b.ScaleInMetricPeriod == 0 {
		b.ScaleInMetricPeriod = 120
	}

	if b.ScaleInMetricEvaluationPeriods == 0 {
		b.ScaleInMetricEvaluationPeriods = 2
	}

	if b.ScaleInMetricThreshold == float64(0) {
		b.ScaleInMetricThreshold = float64(10)
	}

	if b.ScaleInMetricComparisonOperator == "" {
		b.ScaleInMetricComparisonOperator = "LessThanThreshold"
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
