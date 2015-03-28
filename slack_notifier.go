package pudding

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

var (
	errBadSlackResponse = fmt.Errorf("received a response status > 299 from slack")
)

const (
	instSummaryFmt = "_(site=*%s* env=*%s* queue=*%s* role=*%s*)_"
)

// SlackNotifier notifies on slack omgeeeee! ☃
type SlackNotifier struct {
	hookPath, username, icon string
}

// NewSlackNotifier creates a new *SlackNotifier given a team and
// token
func NewSlackNotifier(hookPath, username, icon string) *SlackNotifier {
	return &SlackNotifier{
		hookPath: hookPath,
		username: username,
		icon: func() string {
			if icon == "" {
				return ":travis:"
			}
			return icon
		}(),
	}
}

// Notify sends a notification message (msg) to the given channel,
// which may or may not begin with `#`
func (sn *SlackNotifier) Notify(channel, msg string) error {
	if !strings.HasPrefix("#", channel) {
		channel = fmt.Sprintf("#%s", channel)
	}

	bodyMap := map[string]string{
		"text":       msg,
		"channel":    channel,
		"username":   sn.username,
		"icon_emoji": sn.icon,
	}

	b, err := json.Marshal(bodyMap)
	if err != nil {
		return err
	}

	u := fmt.Sprintf("https://hooks.slack.com/services/%s", sn.hookPath)
	resp, err := http.Post(u, "application/x-www-form-urlencoded", bytes.NewReader(b))
	if err != nil {
		return err
	}

	if resp.StatusCode > 299 {
		return errBadSlackResponse
	}

	return nil
}

// NotificationInstanceSummary returns either an empty string or a summary of relevant bits for use in a
// notification
func NotificationInstanceSummary(inst *Instance) string {
	if inst.Site == "" && inst.Env == "" && inst.Queue == "" && inst.Role == "" {
		return ""
	}

	return fmt.Sprintf(instSummaryFmt, inst.Site, inst.Env, inst.Queue, inst.Role)
}

// NotificationInstanceBuildSummary returns either an empty string or a summary of relevant bits for use in a
// notification
func NotificationInstanceBuildSummary(ib *InstanceBuild) string {
	if ib.Site == "" && ib.Env == "" && ib.Queue == "" && ib.Role == "" {
		return ""
	}

	return fmt.Sprintf(instSummaryFmt, ib.Site, ib.Env, ib.Queue, ib.Role)
}
