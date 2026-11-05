package app_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"codingtutor/internal/app"
	"codingtutor/internal/config"
	"codingtutor/internal/testutil"
)

type testEnv struct {
	server *httptest.Server
	deps   app.Deps
}

func newTestEnv(t *testing.T, opts ...func(*app.Deps)) *testEnv {
	t.Helper()
	deps := app.Deps{
		Config: config.Config{
			Env: "development", SessionTTL: time.Hour, AITimeout: 5 * time.Second,
			AIRateLimit: 100, AIModel: "test-model",
		},
		DB:         testutil.NewDB(t),
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		BcryptCost: bcrypt.MinCost,
	}
	for _, o := range opts {
		o(&deps)
	}
	a, err := app.New(deps)
	if err != nil {
		t.Fatalf("app.New: %v", err)
	}
	srv := httptest.NewServer(a.Handler)
	t.Cleanup(srv.Close)
	return &testEnv{server: srv, deps: deps}
}

// client is a browser-like test client with its own cookie jar.
type client struct {
	t    *testing.T
	env  *testEnv
	http *http.Client
}

func (e *testEnv) newClient(t *testing.T) *client {
	jar, _ := cookiejar.New(nil)
	return &client{t: t, env: e, http: &http.Client{
		Jar: jar,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}}
}

type response struct {
	Status   int
	Body     string
	Location string
}

func (c *client) do(req *http.Request) response {
	c.t.Helper()
	resp, err := c.http.Do(req)
	if err != nil {
		c.t.Fatalf("%s %s: %v", req.Method, req.URL.Path, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return response{Status: resp.StatusCode, Body: string(b), Location: resp.Header.Get("Location")}
}

func (c *client) get(path string) response {
	c.t.Helper()
	req, _ := http.NewRequest("GET", c.env.server.URL+path, nil)
	return c.do(req)
}

func (c *client) getJSON(path string) response {
	c.t.Helper()
	req, _ := http.NewRequest("GET", c.env.server.URL+path, nil)
	req.Header.Set("Accept", "application/json")
	return c.do(req)
}

// csrfToken returns the CSRF token, fetching a page first if needed.
func (c *client) csrfToken() string {
	c.t.Helper()
	u, _ := url.Parse(c.env.server.URL)
	for _, ck := range c.http.Jar.Cookies(u) {
		if ck.Name == "csrf_token" {
			return ck.Value
		}
	}
	c.get("/login")
	for _, ck := range c.http.Jar.Cookies(u) {
		if ck.Name == "csrf_token" {
			return ck.Value
		}
	}
	c.t.Fatal("no csrf cookie issued")
	return ""
}

func (c *client) post(path string, form url.Values) response {
	c.t.Helper()
	if form == nil {
		form = url.Values{}
	}
	if form.Get("csrf_token") == "" {
		form.Set("csrf_token", c.csrfToken())
	}
	req, _ := http.NewRequest("POST", c.env.server.URL+path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return c.do(req)
}

func (c *client) postJSON(path string, form url.Values) response {
	c.t.Helper()
	form.Set("csrf_token", c.csrfToken())
	req, _ := http.NewRequest("POST", c.env.server.URL+path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	return c.do(req)
}

// register creates an account and leaves the client logged in.
func (c *client) register(name, email string) {
	c.t.Helper()
	r := c.post("/register", url.Values{
		"name": {name}, "email": {email}, "password": {"password123"},
		"experience_level": {"beginner"},
	})
	if r.Status != http.StatusSeeOther || r.Location != "/dashboard" {
		c.t.Fatalf("register failed: %d %s\n%s", r.Status, r.Location, r.Body)
	}
}

func mustContain(t *testing.T, body string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(body, w) {
			t.Errorf("response missing %q", w)
		}
	}
}
