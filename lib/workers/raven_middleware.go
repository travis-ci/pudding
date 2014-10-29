package workers

import (
	"github.com/getsentry/raven-go"
	"github.com/jrallison/go-workers"
)

// MiddlewareRaven is the go-workers compatible middleware for
// sentry integration
type MiddlewareRaven struct {
	cl *raven.Client
}

// Call is what does stuff in the middleware stack yey
func (r *MiddlewareRaven) Call(queue string, message *workers.Msg, next func() bool) bool {
	return true
}

// NewMiddlewareRaven builds a *MiddlewareRaven given a sentry DSN
func NewMiddlewareRaven(sentryDSN string) (*MiddlewareRaven, error) {
	cl, err := raven.NewClient(sentryDSN, nil)
	if err != nil {
		return nil, err
	}
	return &MiddlewareRaven{cl: cl}, nil
}
