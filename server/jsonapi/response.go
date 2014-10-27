package jsonapi

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type errResponse struct {
	Errors []*jsonError `json:"errors"`
}

type jsonError struct {
	Details string `json:"details"`
}

func newErrResponse(errors []error) *errResponse {
	r := &errResponse{Errors: []*jsonError{}}
	for _, err := range errors {
		r.Errors = append(r.Errors, &jsonError{Details: err.Error()})
	}

	return r
}

func setContentType(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/vnd.api+json")
}

// Error takes a singular error and responds appropriately
func Error(w http.ResponseWriter, err error, st int) {
	Errors(w, []error{err}, st)
}

// Errors takes an array of errors and responds appropriately
func Errors(w http.ResponseWriter, errors []error, st int) {
	b, err := json.MarshalIndent(newErrResponse(errors), "", "  ")
	if err != nil {
		http.Error(w, "BOOM", http.StatusInternalServerError)
		return
	}

	setContentType(w)
	w.WriteHeader(st)
	fmt.Fprintf(w, string(b)+"\n")
}

// Respond takes an arbitrary thing and `json.MarshalIndent`s it
func Respond(w http.ResponseWriter, thing interface{}, st int) {
	b, err := json.MarshalIndent(thing, "", "  ")
	if err != nil {
		Error(w, err, http.StatusInternalServerError)
		return
	}

	setContentType(w)
	w.WriteHeader(st)
	fmt.Fprintf(w, string(b)+"\n")
}
