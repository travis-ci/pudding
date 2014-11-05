package negroniraven

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/getsentry/raven-go"
	"github.com/meatballhat/logrus"
	"github.com/travis-pro/worker-manager-service/lib"
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

		lib.SendRavenPacket(packet, mw.cl, mw.log)
		panic(p)
	}()

	next(w, req)
}
