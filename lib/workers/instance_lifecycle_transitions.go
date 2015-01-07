package workers

import (
	"encoding/json"
	"fmt"

	"github.com/garyburd/redigo/redis"
	"github.com/jrallison/go-workers"
	"github.com/travis-ci/pudding/lib"
)

var (
	errMissingInstanceLifecycleTransition = fmt.Errorf("missing instance lifecycle transition")
)

func init() {
	defaultQueueFuncs["instance-lifecycle-transitions"] = instanceLifecycleTransitionsMain
}

func instanceLifecycleTransitionsMain(cfg *internalConfig, msg *workers.Msg) {
	iltPayloadJSON := []byte(msg.OriginalJson())
	iltPayload := &lib.InstanceLifecycleTransitionPayload{
		Args: []*lib.InstanceLifecycleTransition{},
	}

	err := json.Unmarshal(iltPayloadJSON, iltPayload)
	if err != nil {
		log.WithField("err", err).Panic("failed to deserialize instance lifecycle transition")
	}

	ilt := iltPayload.InstanceLifecycleTransition()
	if ilt == nil {
		log.WithField("err", errMissingInstanceLifecycleTransition).Panic("no instance lifecycle transition available")
		return
	}

	err = handleInstanceLifecycleTransition(workers.Config.Pool.Get(), ilt)
	if err != nil {
		log.WithField("err", err).Panic("instance lifecycle transition handler returned an error")
	}
}

func handleInstanceLifecycleTransition(rc redis.Conn, ilt *lib.InstanceLifecycleTransition) error {
	// if instance transition set contains instance id
	//   complete lifecycle action with stored action token and hook name
	//   remove instance id from set
	//   remove instance id hash
	// else
	//   short circuit with log message

	log.WithField("ilt", ilt).Info("NOT REALLY handling instance lifecycle transition")
	return nil
}
