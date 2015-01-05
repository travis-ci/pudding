package lib

// SNSMessagePayload is the raw SNS message representation sent to the
// background workers
type SNSMessagePayload struct {
	Args       []*SNSMessage `json:"args"`
	Queue      string        `json:"queue,omitempty"`
	JID        string        `json:"jid,omitempty"`
	Retry      bool          `json:"retry,omitempty"`
	EnqueuedAt float64       `json:"enqueued_at,omitempty"`
}

// SNSMessage returns the SNS message from the args array
func (smp *SNSMessagePayload) SNSMessage() *SNSMessage {
	if len(smp.Args) < 1 {
		return nil
	}

	return smp.Args[0]
}
