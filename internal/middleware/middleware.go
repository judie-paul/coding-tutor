// Package middleware contains HTTP middleware shared across the application.
package middleware

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"codingtutor/internal/users"
)

// Chain applies middleware so the first listed runs outermost.
func Chain(h http.Handler, mw ...func(http.Handler) http.Handler) http.Handler {
	for i := len(mw) - 1; i >= 0; i-- {
		h = mw[i](h)
	}
	return h
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}

// Logging records one structured log line per request.
func Logging(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w}
			next.ServeHTTP(rec, r)
			if rec.status == 0 {
				rec.status = http.StatusOK
			}
			if strings.HasPrefix(r.URL.Path, "/static/") && rec.status < 400 {
				return
			}
			attrs := []any{"method", r.Method, "path", r.URL.Path, "status", rec.status,
				"bytes", rec.bytes, "duration_ms", time.Since(start).Milliseconds()}
			if u := users.FromContext(r.Context()); u != nil {
				attrs = append(attrs, "user_id", u.ID)
			}
			logger.Info("request", attrs...)
		})
	}
}

// Recover turns panics into 500 responses and logs the stack trace.
func Recover(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if v := recover(); v != nil {
					if v == http.ErrAbortHandler {
						panic(v)
					}
					logger.Error("panic", "err", v, "path", r.URL.Path, "stack", string(debug.Stack()))
					w.Header().Set("Connection", "close")
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// SecureHeaders sets conservative security headers on every response.
// When hsts is true (HTTPS deployments) browsers are told to always use HTTPS.
func SecureHeaders(hsts bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return secureHeaders(next, hsts)
	}
}

func secureHeaders(next http.Handler, hsts bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		if hsts {
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		h.Set("Content-Security-Policy",
			"default-src 'self'; img-src 'self' data:; style-src 'self'; script-src 'self'; "+
				"form-action 'self'; frame-ancestors 'none'; base-uri 'self'; object-src 'none'")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}

// LimitBody caps request bodies to n bytes.
func LimitBody(n int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, n)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ---------------------------------------------------------------------------
// CSRF protection (double-submit cookie + Go's cross-origin protection).

const (
	csrfCookie = "csrf_token"
	// CSRFField is the hidden form field carrying the token.
	CSRFField = "csrf_token"
	// CSRFHeader may carry the token for fetch() requests.
	CSRFHeader = "X-CSRF-Token"
)

type csrfKey struct{}

// CSRFToken returns the token to embed in forms for this request.
func CSRFToken(ctx context.Context) string {
	t, _ := ctx.Value(csrfKey{}).(string)
	return t
}

// CSRF rejects state-changing requests unless they carry the token stored in
// the csrf cookie. It also applies net/http's Origin/Sec-Fetch-Site checks.
func CSRF(secure bool, onFail http.Handler) func(http.Handler) http.Handler {
	cop := http.NewCrossOriginProtection()
	cop.SetDenyHandler(onFail)
	return func(next http.Handler) http.Handler {
		inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := ""
			if c, err := r.Cookie(csrfCookie); err == nil && len(c.Value) >= 32 {
				token = c.Value
			}
			fresh := token == ""
			if fresh {
				token = randomToken()
				http.SetCookie(w, &http.Cookie{
					Name: csrfCookie, Value: token, Path: "/",
					HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode,
				})
			}
			if !isSafeMethod(r.Method) {
				sent := r.Header.Get(CSRFHeader)
				if sent == "" {
					sent = r.PostFormValue(CSRFField)
				}
				if fresh || subtle.ConstantTimeCompare([]byte(sent), []byte(token)) != 1 {
					onFail.ServeHTTP(w, r)
					return
				}
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), csrfKey{}, token)))
		})
		return cop.Handler(inner)
	}
}

func isSafeMethod(m string) bool {
	return m == http.MethodGet || m == http.MethodHead || m == http.MethodOptions
}

func randomToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand never fails on supported platforms
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// ---------------------------------------------------------------------------
// Authentication guard.

// RequireAuth redirects anonymous visitors to the login page.
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if users.FromContext(r.Context()) == nil {
			if WantsJSON(r) {
				http.Error(w, `{"error":"Please log in again."}`, http.StatusUnauthorized)
				return
			}
			target := "/login"
			if r.Method == http.MethodGet {
				target += "?next=" + url.QueryEscape(r.URL.RequestURI())
			}
			http.Redirect(w, r, target, http.StatusSeeOther)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

// WantsJSON reports whether the client asked for a JSON response.
func WantsJSON(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept"), "application/json")
}

// ---------------------------------------------------------------------------
// Rate limiting (token bucket per key).

// RateLimiter allows `limit` events per `window` per key, with bursts up to limit.
type RateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	rate    float64 // tokens per second
	burst   float64
	now     func() time.Time
}

type bucket struct {
	tokens float64
	last   time.Time
}

// NewRateLimiter creates a limiter allowing limit events per window.
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		buckets: map[string]*bucket{},
		rate:    float64(limit) / window.Seconds(),
		burst:   float64(limit),
		now:     time.Now,
	}
}

// Allow consumes a token for key, reporting whether the event is allowed.
func (l *RateLimiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: l.burst, last: now}
		l.buckets[key] = b
	}
	b.tokens += now.Sub(b.last).Seconds() * l.rate
	if b.tokens > l.burst {
		b.tokens = l.burst
	}
	b.last = now
	if len(l.buckets) > 10000 {
		l.prune(now)
	}
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// prune drops buckets that have fully refilled. Caller holds the lock.
func (l *RateLimiter) prune(now time.Time) {
	for k, b := range l.buckets {
		if b.tokens+now.Sub(b.last).Seconds()*l.rate >= l.burst {
			delete(l.buckets, k)
		}
	}
}

// Limit applies the limiter to requests, keyed by keyFn. Requests over the
// limit are passed to onLimit.
func Limit(l *RateLimiter, keyFn func(*http.Request) string, onLimit http.Handler) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !l.Allow(keyFn(r)) {
				w.Header().Set("Retry-After", "60")
				onLimit.ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ClientIP returns the request's client IP. When trustProxy is true the
// first X-Forwarded-For entry is used (only enable behind a trusted proxy).
func ClientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			first, _, _ := strings.Cut(xff, ",")
			if ip := strings.TrimSpace(first); ip != "" {
				return ip
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
