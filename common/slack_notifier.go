package common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
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
	bodyMap := map[string]string{
		"channel":    channel,
		"username":   "travisbot",
		"icon_emoji": ":travis:",
	}

	b, err := json.Marshal(bodyMap)
	if err != nil {
		return err
	}

	u := fmt.Sprintf("https://%s.slack.com/services/hooks/hubot?token=%s", sn.team, sn.token)
	_, err = http.Post(u, "application/x-www-form-urlencoded", bytes.NewReader(b))
	return err
}
