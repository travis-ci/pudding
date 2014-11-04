package workers

import (
	"errors"
	"fmt"
	"os"

	"github.com/getsentry/raven-go"
	"github.com/meatballhat/go-workers"
)

// MiddlewareRaven is the go-workers compatible middleware for
// sentry integration
type MiddlewareRaven struct {
	cl *raven.Client
}

// Call is what does stuff in the middleware stack yey.
// It is largely a copy-pasta of the raven CapturePanic func, fwiw.
func (r *MiddlewareRaven) Call(queue string, message *workers.Msg, next func() bool) (ack bool) {
	defer func() {
		var packet *raven.Packet
		p := recover()
		switch rval := p.(type) {
		case nil:
			ack = true
			return
		case error:
			packet = raven.NewPacket(rval.Error(), raven.NewException(rval, raven.NewStacktrace(2, 3, nil)))
		default:
			rvalStr := fmt.Sprint(rval)
			packet = raven.NewPacket(rvalStr, raven.NewException(errors.New(rvalStr), raven.NewStacktrace(2, 3, nil)))
		}

		_, ch := r.cl.Capture(packet, map[string]string{})
		<-ch
		panic(p)
	}()

	ack = next()
	return
}

// Do is a simplified interface used by the mini workers
func (r *MiddlewareRaven) Do(fn func() error) error {
	defer func() {
		var packet *raven.Packet
		p := recover()
		switch rval := p.(type) {
		case nil:
			return
		case error:
			packet = raven.NewPacket(rval.Error(), raven.NewException(rval, raven.NewStacktrace(2, 3, nil)))
		default:
			rvalStr := fmt.Sprint(rval)
			packet = raven.NewPacket(rvalStr, raven.NewException(errors.New(rvalStr), raven.NewStacktrace(2, 3, nil)))
		}

		_, ch := r.cl.Capture(packet, map[string]string{})
		<-ch
		panic(p)
	}()

	return fn()
}

// NewMiddlewareRaven builds a *MiddlewareRaven given a sentry DSN
func NewMiddlewareRaven(sentryDSN string) (*MiddlewareRaven, error) {
	cl, err := raven.NewClient(sentryDSN, map[string]string{
		"level":    "error",
		"logger":   "root",
		"dyno":     os.Getenv("DYNO"),
		"hostname": os.Getenv("HOSTNAME"),
	})
	if err != nil {
		return nil, err
	}
	return &MiddlewareRaven{cl: cl}, nil
}
