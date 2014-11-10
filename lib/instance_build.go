package lib

import (
	"fmt"
	"strings"

	"github.com/gorilla/feeds"
)

var (
	errEmptySite            = fmt.Errorf("empty \"site\" param")
	errInvalidSite          = fmt.Errorf("site must be either org or com")
	errEmptyEnv             = fmt.Errorf("empty \"env\" param")
	errInvalidEnv           = fmt.Errorf("env must be prod, staging, or test")
	errInvalidInstanceCount = fmt.Errorf("count must be more than 0")
	errInvalidState         = fmt.Errorf("state must be pending, started, or finished")
	errEmptyQueue           = fmt.Errorf("empty \"queue\" param")
	errEmptyInstanceType    = fmt.Errorf("empty \"instance_type\" param")
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
	Role         string `json:"role"`
	Site         string `json:"site"`
	Env          string `json:"env"`
	AMI          string `json:"ami"`
	InstanceID   string `json:"instance_id,omitempty"`
	NameTemplate string `json:"name_template,omitempty"`
	InstanceType string `json:"instance_type"`
	SlackChannel string `json:"slack_channel"`
	Count        int    `json:"count"`
	Queue        string `json:"queue"`
	HREF         string `json:"href,omitempty"`
	State        string `json:"state,omitempty"`
	ID           string `json:"id,omitempty"`
}

// NewInstanceBuild creates a new *InstanceBuild, along with
// generating a unique ID and setting the State to "pending"
func NewInstanceBuild() *InstanceBuild {
	return &InstanceBuild{
		ID:    feeds.NewUUID().String(),
		State: "pending",

		// FIXME: accept Role and NameTemplate as configuration
		Role: "worker",
		// XXX: formerly known as:
		//   fmt.Sprintf("travis-%s-%s-%s-%s",
		//     b.Site,
		//     b.Env,
		//     b.Queue,
		//     strings.TrimPrefix(b.InstanceID, "i-"))
		NameTemplate: "travis-{{.Site}}-{{.Env}}-{{.Queue}}-{{.InstanceIDWithoutPrefix}}",
	}
}

// Validate performs multiple validity checks and returns a slice
// of all errors found
func (b *InstanceBuild) Validate() []error {
	errors := []error{}
	if b.Site == "" {
		errors = append(errors, errEmptySite)
	}
	if b.Site != "org" && b.Site != "com" {
		errors = append(errors, errInvalidSite)
	}
	if b.Env == "" {
		errors = append(errors, errEmptyEnv)
	}
	if b.Env != "prod" && b.Env != "staging" && b.Env != "test" {
		errors = append(errors, errInvalidEnv)
	}
	if b.Queue == "" {
		errors = append(errors, errEmptyQueue)
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
