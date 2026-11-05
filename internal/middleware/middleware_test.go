package middleware

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestRateLimiter(t *testing.T) {
	now := time.Unix(0, 0)
	l := NewRateLimiter(3, time.Minute)
	l.now = func() time.Time { return now }

	for i := 0; i < 3; i++ {
		if !l.Allow("a") {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}
	if l.Allow("a") {
		t.Fatal("4th request should be limited")
	}
	if !l.Allow("b") {
		t.Fatal("other keys must have their own bucket")
	}
	now = now.Add(20 * time.Second) // refills one token (3 per 60s)
	if !l.Allow("a") {
		t.Fatal("token should refill over time")
	}
	if l.Allow("a") {
		t.Fatal("only one token should have refilled")
	}
}

func TestCSRF(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(CSRFToken(r.Context())))
	})
	fail := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusForbidden) })
	h := CSRF(false, fail)(ok)

	// A GET issues a token cookie and exposes the token to templates.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || rec.Body.String() != cookies[0].Value {
		t.Fatalf("expected csrf cookie matching context token")
	}
	token := cookies[0]

	post := func(formToken string, withCookie bool) int {
		body := url.Values{CSRFField: {formToken}}.Encode()
		req := httptest.NewRequest("POST", "/", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if withCookie {
			req.AddCookie(token)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}

	if code := post(token.Value, true); code != http.StatusOK {
		t.Errorf("valid token rejected: %d", code)
	}
	if code := post("wrong", true); code != http.StatusForbidden {
		t.Errorf("wrong token accepted: %d", code)
	}
	if code := post(token.Value, false); code != http.StatusForbidden {
		t.Errorf("missing cookie accepted: %d", code)
	}

	// Cross-site browser requests are rejected even with a valid token.
	req := httptest.NewRequest("POST", "/", strings.NewReader(url.Values{CSRFField: {token.Value}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	req.AddCookie(token)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("cross-site request accepted: %d", rec.Code)
	}
}

func TestClientIP(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "10.0.0.1:1234"
	r.Header.Set("X-Forwarded-For", "203.0.113.9, 10.0.0.1")
	if got := ClientIP(r, false); got != "10.0.0.1" {
		t.Errorf("untrusted proxy: got %s", got)
	}
	if got := ClientIP(r, true); got != "203.0.113.9" {
		t.Errorf("trusted proxy: got %s", got)
	}
}
