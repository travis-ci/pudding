package jsonapi

import (
	"encoding/json"
	"fmt"
	"net/http"
)

const (
	errFmt = `{
  "errors": [
    {
      "detail": %q
    }
  ]
}
`
)

func setContentType(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/vnd.api+json")
}

func Error(w http.ResponseWriter, err error, st int) {
	setContentType(w)
	http.Error(w, fmt.Sprintf(errFmt, err.Error()), st)
}

func Respond(w http.ResponseWriter, thing interface{}, st int) {
	b, err := json.MarshalIndent(thing, "", "  ")
	if err != nil {
		Error(w, err, http.StatusInternalServerError)
		return
	}

	setContentType(w)
	fmt.Fprintf(w, string(b)+"\n")
}
