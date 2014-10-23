package workers

import (
	"github.com/getsentry/raven-go"
	"github.com/jrallison/go-workers"
)

type MiddlewareRaven struct {
	cl *raven.Client
}

func (r *MiddlewareRaven) Call(queue string, message *workers.Msg, next func() bool) bool {
	return true
}

func NewMiddlewareRaven(sentryDSN string) (*MiddlewareRaven, error) {
	cl, err := raven.NewClient(sentryDSN, nil)
	if err != nil {
		return nil, err
	}
	return &MiddlewareRaven{cl: cl}, nil
}
