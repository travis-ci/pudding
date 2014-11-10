package negroniraven

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/getsentry/raven-go"
	"github.com/travis-pro/pudding/lib"
)

type Middleware struct {
	cl  *raven.Client
	log *logrus.Logger
}

func NewMiddleware(sentryDSN string) (*Middleware, error) {
	cl, err := raven.NewClient(sentryDSN, nil)
	if err != nil {
		return nil, err
	}

	return &Middleware{cl: cl, log: logrus.New()}, nil
}

func (mw *Middleware) ServeHTTP(w http.ResponseWriter, req *http.Request, next http.HandlerFunc) {
	defer func() {
		var packet *raven.Packet
		tags := map[string]string{"level": "panic"}

		p := recover()
		switch rval := p.(type) {
		case nil:
			return
		case error:
			packet = raven.NewPacket(rval.Error(), raven.NewException(rval, raven.NewStacktrace(2, 3, nil)), raven.NewHttp(req))
		case *logrus.Entry:
			entryErrInterface, ok := rval.Data["err"]
			if !ok {
				entryErrInterface = fmt.Errorf(rval.Message)
			}

			entryErr, ok := entryErrInterface.(error)
			if !ok {
				entryErr = fmt.Errorf(rval.Message)
			}

			packet = raven.NewPacket(rval.Message, raven.NewException(entryErr, raven.NewStacktrace(2, 3, nil)), raven.NewHttp(req))
		default:
			rvalStr := fmt.Sprint(rval)
			packet = raven.NewPacket(rvalStr, raven.NewException(errors.New(rvalStr), raven.NewStacktrace(2, 3, nil)), raven.NewHttp(req))
		}

		lib.SendRavenPacket(packet, mw.cl, mw.log, tags)
		panic(p)
	}()

	next(w, req)
}
