package lib

// Notifier is the interface fulfilled by things like the
// SlackNotifier
type Notifier interface {
	Notify(string, string) error
}
