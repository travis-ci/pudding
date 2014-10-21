package server

import (
	"fmt"
	"net/http"
)

type tokenAuthMiddleware struct {
	Token string
}

func newTokenAuthMiddleware(token string) *tokenAuthMiddleware {
	return &tokenAuthMiddleware{Token: token}
}

func (tam *tokenAuthMiddleware) ServeHTTP(w http.ResponseWriter, req *http.Request, next http.HandlerFunc) {
	if req.Header.Get("Authorization") == ("token "+tam.Token) ||
		req.Header.Get("Authorization") == ("token="+tam.Token) {
		next(w, req)
		return
	}

	fmt.Printf("token=%v header=%#v\n", tam.Token, req.Header)

	w.Header().Set("WWW-Authenticate", "token")
	http.Error(w, "NO", http.StatusUnauthorized)
}
