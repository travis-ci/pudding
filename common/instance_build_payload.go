package common

import "encoding/json"

type InstanceBuildPayload struct {
	Class      string        `json:"class"`
	Args       []interface{} `json:"args"`
	Queue      string        `json:"queue,omitempty"`
	JID        string        `json:"jid,omitempty"`
	Retry      bool          `json:"retry,omitempty"`
	EnqueuedAt float64       `json:"enqueued_at,omitempty"`
}

func (ibp *InstanceBuildPayload) BuildID() string {
	if len(ibp.Args) < 1 {
		return ""
	}

	if buildID, ok := ibp.Args[0].(string); ok {
		return buildID
	}

	return ""
}

func (ibp *InstanceBuildPayload) InstanceBuild() *InstanceBuild {
	if len(ibp.Args) < 2 {
		return nil
	}

	b, err := json.Marshal(ibp.Args[1])
	if err != nil {
		return nil
	}

	ib := &InstanceBuild{}
	err = json.Unmarshal(b, ib)
	if err != nil {
		return nil
	}

	return ib
}
