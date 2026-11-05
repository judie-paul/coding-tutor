package app_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestProtectedPagesRequireLogin(t *testing.T) {
	env := newTestEnv(t)
	c := env.newClient(t)
	for _, p := range []string{"/dashboard", "/settings"} {
		r := c.get(p)
		if r.Status != http.StatusSeeOther || !strings.HasPrefix(r.Location, "/login?next=") {
			t.Errorf("GET %s: expected redirect to login, got %d %q", p, r.Status, r.Location)
		}
	}
}

func TestRegisterLoginLogoutFlow(t *testing.T) {
	env := newTestEnv(t)
	c := env.newClient(t)
	c.register("Grace Hopper", "grace@example.com")

	r := c.get("/dashboard")
	if r.Status != http.StatusOK {
		t.Fatalf("dashboard after register: %d", r.Status)
	}
	mustContain(t, r.Body, "Grace")

	if r := c.post("/logout", nil); r.Status != http.StatusSeeOther {
		t.Fatalf("logout: %d", r.Status)
	}
	if r := c.get("/dashboard"); r.Status != http.StatusSeeOther {
		t.Fatalf("dashboard should require login after logout, got %d", r.Status)
	}

	bad := c.post("/login", url.Values{"email": {"grace@example.com"}, "password": {"nope-nope"}})
	if bad.Status != http.StatusUnauthorized {
		t.Fatalf("bad login: expected 401, got %d", bad.Status)
	}
	mustContain(t, bad.Body, "not correct")

	ok := c.post("/login", url.Values{"email": {"grace@example.com"}, "password": {"password123"}, "next": {"/settings"}})
	if ok.Status != http.StatusSeeOther || ok.Location != "/settings" {
		t.Fatalf("login: %d %q", ok.Status, ok.Location)
	}

	// An attacker-supplied next URL must not cause an open redirect.
	c.post("/logout", nil)
	evil := c.post("/login", url.Values{"email": {"grace@example.com"}, "password": {"password123"}, "next": {"//evil.example"}})
	if evil.Location != "/dashboard" {
		t.Fatalf("open redirect: %q", evil.Location)
	}
}

func TestRegisterShowsValidationErrors(t *testing.T) {
	env := newTestEnv(t)
	c := env.newClient(t)
	r := c.post("/register", url.Values{"name": {""}, "email": {"bad"}, "password": {"x"}})
	if r.Status != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", r.Status)
	}
	mustContain(t, r.Body, "Please enter your name", "valid email", "at least 8 characters")
}

func TestPostWithoutCSRFTokenIsRejected(t *testing.T) {
	env := newTestEnv(t)
	c := env.newClient(t)
	c.csrfToken() // obtain cookie, but send a wrong form token
	r := c.post("/login", url.Values{"csrf_token": {"forged"}, "email": {"a@b.co"}, "password": {"password1"}})
	if r.Status != http.StatusForbidden {
		t.Fatalf("expected 403 for bad CSRF token, got %d", r.Status)
	}
}

func TestSettingsUpdateAndPasswordChange(t *testing.T) {
	env := newTestEnv(t)
	c := env.newClient(t)
	c.register("Linus", "linus@example.com")

	r := c.post("/settings/profile", url.Values{"name": {"Linus T"}, "language_id": {"1"}, "experience_level": {"advanced"}})
	if r.Status != http.StatusSeeOther {
		t.Fatalf("update profile: %d\n%s", r.Status, r.Body)
	}
	page := c.get("/settings")
	mustContain(t, page.Body, "Linus T", "Your profile has been updated")

	// A second device stays logged in until the password changes.
	other := env.newClient(t)
	other.post("/login", url.Values{"email": {"linus@example.com"}, "password": {"password123"}})
	if r := other.get("/dashboard"); r.Status != http.StatusOK {
		t.Fatalf("second device login failed: %d", r.Status)
	}

	r = c.post("/settings/password", url.Values{"current_password": {"password123"}, "new_password": {"newpassword1"}, "confirm_password": {"newpassword1"}})
	if r.Status != http.StatusSeeOther {
		t.Fatalf("change password: %d\n%s", r.Status, r.Body)
	}
	if r := c.get("/dashboard"); r.Status != http.StatusOK {
		t.Fatal("current device should stay logged in after password change")
	}
	if r := other.get("/dashboard"); r.Status != http.StatusSeeOther {
		t.Fatal("other devices must be signed out after password change")
	}
}

func TestDeleteAccount(t *testing.T) {
	env := newTestEnv(t)
	c := env.newClient(t)
	c.register("Temp", "temp@example.com")
	r := c.post("/settings/delete", url.Values{"delete_password": {"password123"}})
	if r.Status != http.StatusSeeOther || r.Location != "/login" {
		t.Fatalf("delete account: %d %q", r.Status, r.Location)
	}
	var n int
	env.deps.DB.QueryRow(`SELECT COUNT(*) FROM users WHERE email = 'temp@example.com'`).Scan(&n)
	if n != 0 {
		t.Fatal("user still exists")
	}
}

func TestSecurityHeadersAndNotFound(t *testing.T) {
	env := newTestEnv(t)
	resp, err := http.Get(env.server.URL + "/does-not-exist")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
	for _, h := range []string{"Content-Security-Policy", "X-Content-Type-Options", "X-Frame-Options"} {
		if resp.Header.Get(h) == "" {
			t.Errorf("missing header %s", h)
		}
	}
}
