package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("APP_ENV", "")
	t.Setenv("SESSION_TTL", "")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Env != "development" || cfg.Addr != ":8080" || cfg.SessionTTL != 7*24*time.Hour ||
		cfg.SecureCookies || !cfg.AIJSONMode || cfg.AIRateLimit != 10 {
		t.Errorf("unexpected defaults: %+v", cfg)
	}
}

func TestProductionRequiresSecureCookies(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("COOKIE_SECURE", "false")
	if _, err := Load(); err == nil {
		t.Fatal("production with insecure cookies must be rejected")
	}
	t.Setenv("COOKIE_SECURE", "")
	cfg, err := Load()
	if err != nil || !cfg.SecureCookies {
		t.Fatalf("production should default to secure cookies: %v %+v", err, cfg)
	}
}

func TestInvalidValues(t *testing.T) {
	cases := map[string]string{
		"APP_ENV":       "staging",
		"SESSION_TTL":   "forever",
		"AI_TIMEOUT":    "-5s",
		"COOKIE_SECURE": "maybe",
		"AI_RATE_LIMIT": "0",
	}
	for key, val := range cases {
		t.Run(key, func(t *testing.T) {
			t.Setenv(key, val)
			if _, err := Load(); err == nil {
				t.Errorf("%s=%q should be rejected", key, val)
			}
		})
	}
}

func TestDotEnvDoesNotOverrideEnvironment(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	os.WriteFile(path, []byte("# comment\nTUTOR_TEST_A=from-file\nTUTOR_TEST_B=\"quoted\"\n"), 0o600)
	t.Setenv("TUTOR_TEST_A", "from-env")
	os.Unsetenv("TUTOR_TEST_B")
	t.Cleanup(func() { os.Unsetenv("TUTOR_TEST_B") })

	if err := loadDotEnv(path); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("TUTOR_TEST_A"); got != "from-env" {
		t.Errorf("environment should win, got %q", got)
	}
	if got := os.Getenv("TUTOR_TEST_B"); got != "quoted" {
		t.Errorf("quotes should be stripped, got %q", got)
	}
	if err := loadDotEnv(filepath.Join(t.TempDir(), "missing")); err != nil {
		t.Errorf("missing .env should not be an error: %v", err)
	}
}
