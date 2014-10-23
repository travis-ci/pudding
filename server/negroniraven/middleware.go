package negroniraven

import (
	"net/http"

	"github.com/getsentry/raven-go"
)

type Middleware struct {
	cl *raven.Client
}

func NewMiddleware(sentryDSN string) (*Middleware, error) {
	cl, err := raven.NewClient(sentryDSN, nil)
	if err != nil {
		return nil, err
	}

	return &Middleware{cl: cl}, nil
}

func (mw *Middleware) ServeHTTP(w http.ResponseWriter, req *http.Request, next http.HandlerFunc) {
	next(w, req)
}
