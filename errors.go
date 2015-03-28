package pudding

import "fmt"

var (
	errEmptyEnv          = fmt.Errorf("empty \"env\" param")
	errEmptyInstanceID   = fmt.Errorf("empty \"instance_id\" param")
	errEmptyInstanceType = fmt.Errorf("empty \"instance_type\" param")
	errEmptyQueue        = fmt.Errorf("empty \"queue\" param")
	errEmptyRole         = fmt.Errorf("empty \"role\" param")
	errEmptyRoleARN      = fmt.Errorf("empty \"role_arn\" param")
	errEmptySite         = fmt.Errorf("empty \"site\" param")
	errEmptyTopicARN     = fmt.Errorf("empty \"topic_arn\" param")

	errInvalidInstanceCount = fmt.Errorf("count must be more than 0")
	errInvalidState         = fmt.Errorf("state must be pending, started, or finished")
)
