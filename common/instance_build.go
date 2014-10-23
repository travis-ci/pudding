package common

import "fmt"

var (
	errEmptySite         = fmt.Errorf("empty \"site\" param")
	errInvalidSite       = fmt.Errorf("site must be either org or com")
	errEmptyEnv          = fmt.Errorf("empty \"env\" param")
	errInvalidEnv        = fmt.Errorf("env must be either prod or staging")
	errEmptyQueue        = fmt.Errorf("empty \"queue\" param")
	errEmptyInstanceType = fmt.Errorf("empty \"instance_type\" param")
)

type InstanceBuildsCollectionSingular struct {
	InstanceBuilds *InstanceBuild `json:"instance_builds"`
}

type InstanceBuildsCollection struct {
	InstanceBuilds []*InstanceBuild `json:"instance_builds"`
}

type InstanceBuild struct {
	Site         string `json:"site"`
	Env          string `json:"env"`
	AMI          string `json:"ami"`
	InstanceType string `json:"instance_type"`
	Count        int    `json:"count"`
	Queue        string `json:"queue"`
	HREF         string `json:"href,omitempty"`
	ID           string `json:"id,omitempty"`
}

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
	if b.Env != "prod" && b.Env != "staging" {
		errors = append(errors, errInvalidEnv)
	}
	if b.Queue == "" {
		errors = append(errors, errEmptyQueue)
	}
	if b.InstanceType == "" {
		errors = append(errors, errEmptyInstanceType)
	}

	return errors
}

func (b *InstanceBuild) UpdateFromDetails(d *InstanceBuildDetails) {
	if d.ID != "" {
		b.ID = d.ID
	}

	return
}
