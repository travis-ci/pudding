package workers

import "github.com/mitchellh/goamz/aws"

type config struct {
	AWSAuth   aws.Auth
	AWSRegion aws.Region
}
