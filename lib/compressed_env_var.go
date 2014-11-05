package lib

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
)

var (
	// ErrMissingEnvVar is used to signal when an env var is missing
	// :boom:
	ErrMissingEnvVar = fmt.Errorf("missing env var")
)

// GetCompressedEnvVar looks up an env var and base64-decodes and
// gunzips it if present
func GetCompressedEnvVar(key string) (string, error) {
	value := os.Getenv(key)
	if value == "" {
		return "", ErrMissingEnvVar
	}

	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return "", err
	}

	r, err := gzip.NewReader(bytes.NewReader(decoded))
	if err != nil {
		return "", err
	}

	b, err := ioutil.ReadAll(r)
	if err != nil {
		return "", err
	}

	return string(b), nil
}
