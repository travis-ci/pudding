package pudding

import "encoding/json"

// SNSMessage is totally an SNS message, eh
type SNSMessage struct {
	Message           string
	MessageID         string `json:"MessageId"`
	Signature         string
	SignatureVersion  string
	SigningCertURL    string
	Subject           string
	SubscribeURL      string
	Timestamp         string
	Token             string
	TopicARN          string `json:"TopicArn"`
	Type              string
	UnsubscribeURL    string
	MessageAttributes map[string]*SNSMessageAttribute
}

// SNSMessageAttribute is what shows up in MessageAttributes
type SNSMessageAttribute struct {
	Type  string
	Value string
}

// NewSNSMessage makes a new SNSMessage with empty MessageAttributes map
func NewSNSMessage() *SNSMessage {
	return &SNSMessage{
		MessageAttributes: map[string]*SNSMessageAttribute{},
	}
}

// AutoscalingLifecycleAction attempts to unmarshal the message payload into an *AutoscalingLifecycleAction
func (m *SNSMessage) AutoscalingLifecycleAction() (*AutoscalingLifecycleAction, error) {
	a := &AutoscalingLifecycleAction{}
	err := json.Unmarshal([]byte(m.Message), a)
	if err != nil {
		return nil, err
	}

	return a, nil
}
