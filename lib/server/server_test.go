package server

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/travis-ci/pudding/lib"
)

var (
	defaultTestAuthToken = "swordfish"
)

func init() {
	lib.RedisNamespace = "pudding-test"
}

func buildTestConfig() *Config {
	return &Config{
		Addr:      ":17321",
		AuthToken: defaultTestAuthToken,
		Debug:     true,
		RedisURL: func() string {
			v := os.Getenv("REDIS_URL")
			if v == "" {
				v = "redis://localhost:6379/0"
			}
			return v
		}(),
	}
}

func buildTestServer(cfg *Config) *server {
	if cfg == nil {
		cfg = buildTestConfig()
	}

	srv, err := newServer(cfg)
	if err != nil {
		panic(err)
	}

	srv.Setup()
	return srv
}

func makeRequest(method, path string, body io.Reader) *httptest.ResponseRecorder {
	return makeRequestWithHeaders(method, path, body, map[string]string{})
}

func makeAuthenticatedRequest(method, path string, body io.Reader) *httptest.ResponseRecorder {
	return makeRequestWithHeaders(method, path, body,
		map[string]string{"Authorization": fmt.Sprintf("token %s", defaultTestAuthToken)})
}

func makeRequestWithHeaders(method, path string, body io.Reader, headers map[string]string) *httptest.ResponseRecorder {
	srv := buildTestServer(nil)
	// make the shutdown channel buffered so that we don't get deadlock in tests
	srv.s.Shutdown = make(chan bool, 2)

	if body == nil {
		body = bytes.NewReader([]byte(""))
	}
	req, err := http.NewRequest(method, fmt.Sprintf("http://example.com%s", path), body)
	if err != nil {
		panic(err)
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	return w
}

func makeTestAutoscalingGroupBuildRequest() io.Reader {
	return bytes.NewReader([]byte(`{
    "autoscaling_group_builds": {
      "site": "com",
      "env": "prod",
      "queue": "fancy",
      "role": "worky",
      "instance_id": "i-abcd123",
      "role_arn": "arn:aws:iam::1234567899:role/pudding-test-foo",
      "topic_arn": "arn:aws:sns:us-east-1::1234567899:pudding-test-foo",
      "min_size": 1,
      "max_size": 10,
      "desired_capacity": 1,
      "default_cooldown": 1200,
      "instance_type": "c3.4xlarge"
    }
  }`))
}

func assertStatus(t *testing.T, expected, actual int) {
	if actual != expected {
		t.Errorf("response status %v != %v", actual, expected)
	}
}

func assertBody(t *testing.T, expected, actual string) {
	if actual != expected {
		t.Errorf("response body %q != %q", actual, expected)
	}
}

func TestGetOhai(t *testing.T) {
	w := makeRequest("GET", "/", nil)
	assertStatus(t, 200, w.Code)
	assertBody(t, "ohai\n", w.Body.String())
}

func TestShutdown(t *testing.T) {
	w := makeAuthenticatedRequest("DELETE", "/", nil)
	assertStatus(t, 204, w.Code)
}

func TestExpvars(t *testing.T) {
	w := makeAuthenticatedRequest("GET", "/debug/vars", nil)
	assertStatus(t, 200, w.Code)
}

func TestKaboom(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatalf("kaboom did not panic")
		}
	}()
	makeAuthenticatedRequest("POST", "/kaboom", nil)
}

func TestCreateAutoscalingGroupBuild(t *testing.T) {
	w := makeAuthenticatedRequest("POST", "/autoscaling-group-builds", nil)
	assertStatus(t, 400, w.Code)

	w = makeAuthenticatedRequest("POST", "/autoscaling-group-builds", makeTestAutoscalingGroupBuildRequest())
	assertStatus(t, 202, w.Code)
}
