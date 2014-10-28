package common

import (
	"fmt"
	"net/http"
	"net/url"
)

type SlackNotifier struct {
	url   string
	token string
}

func NewSlackNotifier(url, token string) *SlackNotifier {
	return &SlackNotifier{
		url:   url,
		token: token,
	}
}

func (sn *SlackNotifier) Notify(channel, msg string) error {
	v := &url.Values{}
	v.Add("token", sn.token)
	v.Add("channel", channel)
	v.Add("username", "worker manager")
	v.Add("icon_emoji", ":see_no_evil:")
	_, err := http.Post(fmt.Sprintf("%s/api/chat.postMessage?%s", sn.url, v.Encode()),
		"application/x-www-form-urlencoded", nil)
	return err
}
