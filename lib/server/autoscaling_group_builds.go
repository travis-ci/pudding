package server

import (
	"encoding/json"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/travis-ci/pudding/lib"
	"github.com/travis-ci/pudding/lib/db"
)

type autoscalingGroupBuilder struct {
	QueueName string
	r         *redis.Pool
}

func newAutoscalingGroupBuilder(r *redis.Pool, queueName string) (*autoscalingGroupBuilder, error) {
	return &autoscalingGroupBuilder{
		QueueName: queueName,

		r: r,
	}, nil
}

func (asgb *autoscalingGroupBuilder) Build(b *lib.AutoscalingGroupBuild) (*lib.AutoscalingGroupBuild, error) {
	conn := asgb.r.Get()
	defer func() { _ = conn.Close() }()

	buildPayload := &lib.AutoscalingGroupBuildPayload{
		Args:       []*lib.AutoscalingGroupBuild{b},
		Queue:      asgb.QueueName,
		JID:        b.ID,
		Retry:      false,
		EnqueuedAt: float64(time.Now().UTC().Unix()),
	}

	buildPayloadJSON, err := json.Marshal(buildPayload)
	if err != nil {
		return nil, err
	}

	err = db.EnqueueJob(conn, asgb.QueueName, string(buildPayloadJSON))
	return b, err
}
