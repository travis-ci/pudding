package workers

import (
	"fmt"
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/getsentry/raven-go"
	"github.com/jrallison/go-workers"
	"github.com/travis-pro/pudding/lib"
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
		case *logrus.Entry:
			entryErrInterface, ok := rval.Data["err"]
			if !ok {
				entryErrInterface = fmt.Errorf(rval.Message)
			}

			entryErr, ok := entryErrInterface.(error)
			if !ok {
				entryErr = fmt.Errorf(rval.Message)
			}

			packet = raven.NewPacket(rval.Message, raven.NewException(entryErr, raven.NewStacktrace(2, 3, nil)))
		default:
			rvalStr := fmt.Sprint(rval)
			packet = raven.NewPacket(rvalStr, raven.NewException(fmt.Errorf(rvalStr), raven.NewStacktrace(2, 3, nil)))
		}

		lib.SendRavenPacket(packet, r.cl, log, nil)
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
			errMsg := rval.Error()
			if errMsg == "" {
				errMsg = "generic worker error (?)"
			}
			packet = raven.NewPacket(errMsg, raven.NewException(rval, raven.NewStacktrace(2, 3, nil)))
		case *logrus.Entry:
			entryErrInterface, ok := rval.Data["err"]
			if !ok {
				entryErrInterface = fmt.Errorf(rval.Message)
			}

			entryErr, ok := entryErrInterface.(error)
			if !ok {
				entryErr = fmt.Errorf(rval.Message)
			}

			packet = raven.NewPacket(rval.Message, raven.NewException(entryErr, raven.NewStacktrace(2, 3, nil)))
		default:
			rvalStr := fmt.Sprint(rval)
			if rvalStr == "" {
				rvalStr = "generic worker error (?)"
			}
			packet = raven.NewPacket(rvalStr, raven.NewException(fmt.Errorf(rvalStr), raven.NewStacktrace(2, 3, nil)))
		}

		lib.SendRavenPacket(packet, r.cl, log, nil)
		panic(p)
	}()

	return fn()
}

// NewMiddlewareRaven builds a *MiddlewareRaven given a sentry DSN
func NewMiddlewareRaven(sentryDSN string) (*MiddlewareRaven, error) {
	cl, err := raven.NewClient(sentryDSN, map[string]string{
		"level":    "panic",
		"logger":   "root",
		"dyno":     os.Getenv("DYNO"),
		"hostname": os.Getenv("HOSTNAME"),
	})
	if err != nil {
		return nil, err
	}
	return &MiddlewareRaven{cl: cl}, nil
}
