// Package auth implements server-side sessions and the login, registration
// and logout flows.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"codingtutor/internal/database"
	"codingtutor/internal/users"
)

// CookieName is the session cookie's name.
const CookieName = "tutor_session"

// Sessions stores login sessions in SQLite. Only a SHA-256 hash of each
// token is stored, so a leaked database cannot be used to hijack sessions.
type Sessions struct {
	db     *sql.DB
	users  *users.Store
	ttl    time.Duration
	secure bool
	now    func() time.Time
	logger *slog.Logger
}

// NewSessions creates a session manager.
func NewSessions(db *sql.DB, us *users.Store, ttl time.Duration, secure bool, logger *slog.Logger) *Sessions {
	return &Sessions{db: db, users: us, ttl: ttl, secure: secure, now: time.Now, logger: logger}
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func newToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// Create starts a new session for userID and sets the session cookie. Any
// session already present on the request is destroyed first (prevents
// session fixation).
func (s *Sessions) Create(ctx context.Context, w http.ResponseWriter, r *http.Request, userID int64) error {
	if c, err := r.Cookie(CookieName); err == nil {
		s.deleteByToken(ctx, c.Value)
	}
	token, err := newToken()
	if err != nil {
		return fmt.Errorf("generate session token: %w", err)
	}
	now := s.now()
	expires := now.Add(s.ttl)
	_, err = s.db.ExecContext(ctx, `INSERT INTO sessions (token_hash, user_id, expires_at, created_at, last_seen_at)
		VALUES (?, ?, ?, ?, ?)`, hashToken(token), userID, database.FormatTime(expires),
		database.FormatTime(now), database.FormatTime(now))
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	http.SetCookie(w, &http.Cookie{
		Name: CookieName, Value: token, Path: "/", Expires: expires,
		MaxAge: int(s.ttl.Seconds()), HttpOnly: true, Secure: s.secure, SameSite: http.SameSiteLaxMode,
	})
	return nil
}

// Destroy ends the request's session and clears the cookie.
func (s *Sessions) Destroy(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(CookieName); err == nil {
		s.deleteByToken(ctx, c.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name: CookieName, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: s.secure, SameSite: http.SameSiteLaxMode,
	})
}

// DestroyOthers ends every session for userID except the request's own.
// Used after a password change.
func (s *Sessions) DestroyOthers(ctx context.Context, r *http.Request, userID int64) error {
	keep := ""
	if c, err := r.Cookie(CookieName); err == nil {
		keep = hashToken(c.Value)
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ? AND token_hash <> ?`, userID, keep)
	return err
}

func (s *Sessions) deleteByToken(ctx context.Context, token string) {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, hashToken(token)); err != nil {
		s.logger.Error("delete session", "err", err)
	}
}

// UserForToken returns the user owning a valid, unexpired session token.
func (s *Sessions) UserForToken(ctx context.Context, token string) (*users.User, error) {
	h := hashToken(token)
	var userID int64
	var lastSeen string
	err := s.db.QueryRowContext(ctx, `SELECT user_id, last_seen_at FROM sessions WHERE token_hash = ? AND expires_at > ?`,
		h, database.FormatTime(s.now())).Scan(&userID, &lastSeen)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, users.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	// Record activity at most every five minutes to avoid a write per request.
	if s.now().Sub(database.ParseTime(lastSeen)) > 5*time.Minute {
		if _, err := s.db.ExecContext(ctx, `UPDATE sessions SET last_seen_at = ? WHERE token_hash = ?`,
			database.FormatTime(s.now()), h); err != nil {
			s.logger.Warn("update session last_seen", "err", err)
		}
	}
	return s.users.GetByID(ctx, userID)
}

// LoadUser attaches the logged-in user (if any) to the request context.
func (s *Sessions) LoadUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(CookieName)
		if err != nil || c.Value == "" {
			next.ServeHTTP(w, r)
			return
		}
		u, err := s.UserForToken(r.Context(), c.Value)
		switch {
		case err == nil:
			r = r.WithContext(users.WithUser(r.Context(), u))
		case errors.Is(err, users.ErrNotFound):
			// Expired or unknown session: clear the stale cookie.
			http.SetCookie(w, &http.Cookie{Name: CookieName, Value: "", Path: "/", MaxAge: -1,
				HttpOnly: true, Secure: s.secure, SameSite: http.SameSiteLaxMode})
		default:
			s.logger.Error("load session", "err", err)
		}
		next.ServeHTTP(w, r)
	})
}

// DeleteExpired removes expired sessions and returns how many were removed.
func (s *Sessions) DeleteExpired(ctx context.Context) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= ?`, database.FormatTime(s.now()))
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// StartCleanup deletes expired sessions periodically until ctx is cancelled.
func (s *Sessions) StartCleanup(ctx context.Context, every time.Duration) {
	go func() {
		t := time.NewTicker(every)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if n, err := s.DeleteExpired(ctx); err != nil {
					s.logger.Error("session cleanup", "err", err)
				} else if n > 0 {
					s.logger.Info("expired sessions removed", "count", n)
				}
			}
		}
	}()
}
