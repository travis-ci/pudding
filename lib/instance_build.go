package lib

import (
	"strings"

	"github.com/gorilla/feeds"
)

// InstanceBuildsCollectionSingular is the singular representation
// used in jsonapi bodies
type InstanceBuildsCollectionSingular struct {
	InstanceBuilds *InstanceBuild `json:"instance_builds"`
}

// InstanceBuildsCollection is the collection representation used
// in jsonapi bodies
type InstanceBuildsCollection struct {
	InstanceBuilds []*InstanceBuild `json:"instance_builds"`
}

// InstanceBuild contains everything needed by a background worker
// to build the instance
type InstanceBuild struct {
	Role            string `json:"role,omitempty"`
	Site            string `json:"site"`
	Env             string `json:"env"`
	AMI             string `json:"ami"`
	InstanceID      string `json:"instance_id,omitempty"`
	NameTemplate    string `json:"name_template,omitempty"`
	InstanceType    string `json:"instance_type"`
	SlackChannel    string `json:"slack_channel"`
	Count           int    `json:"count"`
	Queue           string `json:"queue"`
	SubnetID        string `json:"subnet_id,omitempty"`
	SecurityGroupID string `json:"security_group_id,omitempty"`
	HREF            string `json:"href,omitempty"`
	State           string `json:"state,omitempty"`
	ID              string `json:"id,omitempty"`
}

// NewInstanceBuild creates a new *InstanceBuild, along with
// generating a unique ID and setting the State to "pending"
func NewInstanceBuild() *InstanceBuild {
	return &InstanceBuild{
		ID: feeds.NewUUID().String(),
	}
}

// Hydrate is used to overwrite "null" defaults that result from
// serialize/deserialize via JSON
func (b *InstanceBuild) Hydrate() {
	if b.State == "" {
		b.State = "pending"
	}

	if b.Role == "" {
		b.Role = "worker"
	}

	if b.NameTemplate == "" {
		b.NameTemplate = "{{.Role}}-{{.Site}}-{{.Env}}-{{.Queue}}-{{.InstanceIDWithoutPrefix}}"
	}
}

// Validate performs multiple validity checks and returns a slice
// of all errors found
func (b *InstanceBuild) Validate() []error {
	errors := []error{}
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
	if b.InstanceType == "" {
		errors = append(errors, errEmptyInstanceType)
	}
	if b.State != "pending" && b.State != "started" && b.State != "finished" {
		errors = append(errors, errInvalidState)
	}
	if b.Count < 1 {
		errors = append(errors, errInvalidInstanceCount)
	}

	return errors
}

// InstanceIDWithoutPrefix returns the InstanceID without "i-"
func (b *InstanceBuild) InstanceIDWithoutPrefix() string {
	return strings.TrimPrefix(b.InstanceID, "i-")
}
