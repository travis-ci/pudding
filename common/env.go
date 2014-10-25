package common

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
)

var (
	errMissingEnvVar = fmt.Errorf("missing env var")
)

func GetCompressedEnvVar(key string) (string, error) {
	value := os.Getenv(key)
	if value == "" {
		return "", errMissingEnvVar
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
