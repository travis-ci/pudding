package workers

import (
	"encoding/json"

	"github.com/jrallison/go-workers"
	"github.com/mitchellh/goamz/aws"
	"github.com/mitchellh/goamz/ec2"
	"github.com/travis-pro/worker-manager-service/common"
)

func instanceBuildsMain(msg *workers.Msg) {
	buildPayloadJSON := []byte(msg.OriginalJson())
	buildPayload := &common.InstanceBuildPayload{}

	err := json.Unmarshal(buildPayloadJSON, buildPayload)
	if err != nil {
		log.WithField("err", err).Error("failed to deserialize message")
	}

	ibw := newInstanceBuilderWorker(cfg.AWSAuth, cfg.AWSRegion)
	ibw.Build(buildPayload.InstanceBuild())
}

type instanceBuilderWorker struct {
	ec2 *ec2.EC2
}

func newInstanceBuilderWorker(auth aws.Auth, region aws.Region) *instanceBuilderWorker {
	return &instanceBuilderWorker{
		ec2: ec2.New(auth, region),
	}
}

func (ibw *instanceBuilderWorker) Build(b *common.InstanceBuild) {
	log.WithField("build", b).Info("not really building this")
	// TODO: port the guts of the thing
}
