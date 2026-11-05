// Package config loads application settings from environment variables.
package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all runtime settings for the application.
type Config struct {
	Env        string // "development" or "production"
	Addr       string // listen address, e.g. ":8080"
	DBPath     string // path to the SQLite database file
	LogLevel   string // debug, info, warn, error
	SessionTTL time.Duration
	// SecureCookies marks cookies as Secure (HTTPS only). Defaults to true in production.
	SecureCookies bool
	// TrustProxy uses X-Forwarded-For for the client IP. Only enable behind a trusted reverse proxy.
	TrustProxy bool

	AIBaseURL   string // OpenAI-compatible base URL, e.g. https://api.openai.com/v1
	AIAPIKey    string // never sent to the browser
	AIModel     string
	AITimeout   time.Duration
	AIJSONMode  bool // send response_format=json_object for structured requests
	AIRateLimit int  // AI requests allowed per user per minute
}

// IsProduction reports whether the app runs in production mode.
func (c Config) IsProduction() bool { return c.Env == "production" }

// AIConfigured reports whether enough AI settings exist to make API calls.
func (c Config) AIConfigured() bool { return c.AIBaseURL != "" && c.AIModel != "" }

// Load reads configuration from the environment. Values in a local .env file
// (if present) are applied first but never override real environment variables.
func Load() (Config, error) {
	if err := loadDotEnv(".env"); err != nil {
		return Config{}, err
	}

	env := getenv("APP_ENV", "development")
	if env != "development" && env != "production" {
		return Config{}, fmt.Errorf("APP_ENV must be development or production, got %q", env)
	}

	cfg := Config{
		Env:       env,
		Addr:      getenv("APP_ADDR", ":8080"),
		DBPath:    getenv("DB_PATH", "data/tutor.db"),
		LogLevel:  getenv("LOG_LEVEL", "info"),
		AIBaseURL: strings.TrimRight(getenv("AI_BASE_URL", "https://api.openai.com/v1"), "/"),
		AIAPIKey:  os.Getenv("AI_API_KEY"),
		AIModel:   getenv("AI_MODEL", "gpt-4o-mini"),
	}

	var err error
	if cfg.SessionTTL, err = getDuration("SESSION_TTL", 7*24*time.Hour); err != nil {
		return Config{}, err
	}
	if cfg.AITimeout, err = getDuration("AI_TIMEOUT", 60*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.SecureCookies, err = getBool("COOKIE_SECURE", env == "production"); err != nil {
		return Config{}, err
	}
	if cfg.TrustProxy, err = getBool("TRUST_PROXY", false); err != nil {
		return Config{}, err
	}
	if cfg.AIJSONMode, err = getBool("AI_JSON_MODE", true); err != nil {
		return Config{}, err
	}
	if cfg.AIRateLimit, err = getInt("AI_RATE_LIMIT", 10); err != nil {
		return Config{}, err
	}
	if cfg.AIRateLimit < 1 {
		return Config{}, errors.New("AI_RATE_LIMIT must be at least 1")
	}
	if cfg.IsProduction() && !cfg.SecureCookies {
		return Config{}, errors.New("COOKIE_SECURE must be true in production")
	}
	return cfg, nil
}

func getenv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return fallback
}

func getDuration(key string, fallback time.Duration) (time.Duration, error) {
	v := getenv(key, "")
	if v == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration like 30s or 24h", key)
	}
	return d, nil
}

func getBool(key string, fallback bool) (bool, error) {
	v := getenv(key, "")
	if v == "" {
		return fallback, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("%s must be true or false", key)
	}
	return b, nil
}

func getInt(key string, fallback int) (int, error) {
	v := getenv(key, "")
	if v == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("%s must be a whole number", key)
	}
	return n, nil
}

// loadDotEnv reads KEY=VALUE lines from path into the process environment.
// Existing environment variables win. A missing file is not an error.
func loadDotEnv(path string) error {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.Trim(strings.TrimSpace(val), `"'`)
		if _, exists := os.LookupEnv(key); !exists {
			os.Setenv(key, val)
		}
	}
	return sc.Err()
}
