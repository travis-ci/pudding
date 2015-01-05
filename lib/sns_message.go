package lib

// SNSMessage is totally an SNS message, eh
type SNSMessage struct {
	Type             string
	MessageID        string `json:"MessageId"`
	TopicARN         string `json:"TopicArn"`
	Subject          string
	Message          string
	Timestamp        string
	SignatureVersion string
	Signature        string
	SigningCertURL   string
	UnsubscribeURL   string
}
