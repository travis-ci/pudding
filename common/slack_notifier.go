package common

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

type SlackNotifier struct {
	team  string
	token string
}

func NewSlackNotifier(team, token string) *SlackNotifier {
	return &SlackNotifier{
		team:  team,
		token: token,
	}
}

func (sn *SlackNotifier) Notify(channel, msg string) error {
	if !strings.HasPrefix("#", channel) {
		channel = fmt.Sprintf("#%s", channel)
	}

	bodyMap := map[string]string{
		"text":       msg,
		"channel":    channel,
		"username":   "travisbot",
		"icon_emoji": ":travis:",
	}

	b, err := json.Marshal(bodyMap)
	if err != nil {
		return err
	}

	u := fmt.Sprintf("https://%s.slack.com/services/hooks/hubot?token=%s", sn.team, sn.token)
	resp, err := http.Post(u, "application/x-www-form-urlencoded", bytes.NewReader(b))
	if err != nil {
		return err
	}

	if resp.StatusCode > 299 {
		return errBadSlackResponse
	}

	return nil
}
