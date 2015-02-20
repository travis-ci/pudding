package server

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/garyburd/redigo/redis"
	"github.com/goamz/goamz/ec2"
	"github.com/travis-ci/pudding/lib"
	"github.com/travis-ci/pudding/lib/db"
)

var (
	defaultTestAuthToken  = "swordfish"
	defaultTestInstanceID = "i-abcd123"
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

func collapsedJSON(s string) string {
	out := []string{}
	for _, part := range strings.Split(s, "\n") {
		for _, subpart := range strings.Split(part, " ") {
			out = append(out, strings.TrimSpace(subpart))
		}
	}
	return strings.Join(out, "")
}

func ensureExampleDataPresent(redisURL string) {
	u, err := url.Parse(redisURL)
	if err != nil {
		panic(err)
	}

	conn, err := redis.Dial("tcp", u.Host)
	if err != nil {
		panic(err)
	}

	err = db.StoreInstances(conn, map[string]ec2.Instance{
		defaultTestInstanceID: ec2.Instance{
			InstanceId:       defaultTestInstanceID,
			InstanceType:     "c3.2xlarge",
			ImageId:          "ami-abcd123",
			IPAddress:        "",
			PrivateIPAddress: "10.0.0.1",
			LaunchTime:       "1955-11-05T21:30:19+0800",
		},
	}, 300)
	if err != nil {
		panic(err)
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

	ensureExampleDataPresent(cfg.RedisURL)
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

func assertNotBody(t *testing.T, notExpected, actual string) {
	if actual == notExpected {
		t.Errorf("response body %q == %q", actual, notExpected)
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

func TestGetInstances(t *testing.T) {
	w := makeAuthenticatedRequest("GET", "/instances", nil)
	assertStatus(t, 200, w.Code)
	assertNotBody(t, `{"instances":[]}`, collapsedJSON(w.Body.String()))
}

func TestGetInstanceByID(t *testing.T) {
	w := makeAuthenticatedRequest("GET", "/instances/i-bogus123", nil)
	assertStatus(t, 200, w.Code)
	assertBody(t, `{"instances":[]}`, collapsedJSON(w.Body.String()))

	w = makeAuthenticatedRequest("GET", fmt.Sprintf("/instances/%s", defaultTestInstanceID), nil)
	assertStatus(t, 200, w.Code)
	assertNotBody(t, `{"instances":[]}`, collapsedJSON(w.Body.String()))
}

func TestDeleteInstanceByID(t *testing.T) {
	w := makeAuthenticatedRequest("DELETE", "/instances/i-bogus123", nil)
	assertStatus(t, 202, w.Code)
	assertBody(t, `{"ok":"workingonthat"}`, collapsedJSON(w.Body.String()))
}
