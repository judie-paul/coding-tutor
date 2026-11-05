package auth

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"codingtutor/internal/testutil"
	"codingtutor/internal/users"
)

func setup(t *testing.T) (*Sessions, *users.User) {
	t.Helper()
	db := testutil.NewDB(t)
	store := users.NewStore(db)
	u, err := users.NewService(store, bcrypt.MinCost).Register(context.Background(),
		users.RegisterInput{Name: "Sam", Email: "sam@example.com", Password: "password1"})
	if err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewSessions(db, store, time.Hour, true, logger), u
}

func sessionCookie(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == CookieName {
			return c
		}
	}
	t.Fatal("no session cookie set")
	return nil
}

func TestSessionLifecycle(t *testing.T) {
	s, u := setup(t)
	ctx := context.Background()

	rec := httptest.NewRecorder()
	if err := s.Create(ctx, rec, httptest.NewRequest("POST", "/login", nil), u.ID); err != nil {
		t.Fatal(err)
	}
	c := sessionCookie(t, rec)
	if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode {
		t.Errorf("cookie flags not secure: %+v", c)
	}

	got, err := s.UserForToken(ctx, c.Value)
	if err != nil || got.ID != u.ID {
		t.Fatalf("UserForToken: %v %v", got, err)
	}

	// The raw token must never be stored.
	var n int
	s.db.QueryRow(`SELECT COUNT(*) FROM sessions WHERE token_hash = ?`, c.Value).Scan(&n)
	if n != 0 {
		t.Error("raw session token found in database")
	}

	req := httptest.NewRequest("POST", "/logout", nil)
	req.AddCookie(c)
	s.Destroy(ctx, httptest.NewRecorder(), req)
	if _, err := s.UserForToken(ctx, c.Value); !errors.Is(err, users.ErrNotFound) {
		t.Fatalf("session should be invalid after logout, got %v", err)
	}
}

func TestSessionExpiry(t *testing.T) {
	s, u := setup(t)
	ctx := context.Background()
	rec := httptest.NewRecorder()
	if err := s.Create(ctx, rec, httptest.NewRequest("POST", "/", nil), u.ID); err != nil {
		t.Fatal(err)
	}
	token := sessionCookie(t, rec).Value

	s.now = func() time.Time { return time.Now().Add(2 * time.Hour) }
	if _, err := s.UserForToken(ctx, token); !errors.Is(err, users.ErrNotFound) {
		t.Fatalf("expired session accepted: %v", err)
	}
	if n, err := s.DeleteExpired(ctx); err != nil || n != 1 {
		t.Fatalf("DeleteExpired = %d, %v", n, err)
	}
}

func TestCreateReplacesExistingSession(t *testing.T) {
	s, u := setup(t)
	ctx := context.Background()
	rec := httptest.NewRecorder()
	s.Create(ctx, rec, httptest.NewRequest("POST", "/", nil), u.ID)
	old := sessionCookie(t, rec)

	req := httptest.NewRequest("POST", "/login", nil)
	req.AddCookie(old)
	s.Create(ctx, httptest.NewRecorder(), req, u.ID)
	if _, err := s.UserForToken(ctx, old.Value); err == nil {
		t.Fatal("old session should be destroyed on new login (session fixation)")
	}
}

func TestSafeNext(t *testing.T) {
	cases := map[string]string{
		"":                   "/dashboard",
		"/tutor":             "/tutor",
		"//evil.com":         "/dashboard",
		"https://evil.com":   "/dashboard",
		"/\\evil.com":        "/dashboard",
		"/quizzes?x=1":       "/quizzes?x=1",
		"/a\r\nSet-Cookie:x": "/dashboard",
	}
	for in, want := range cases {
		if got := SafeNext(in); got != want {
			t.Errorf("SafeNext(%q) = %q, want %q", in, got, want)
		}
	}
}
