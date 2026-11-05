// Package users manages student accounts: registration, credentials and profiles.
package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/mail"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"

	"codingtutor/internal/database"
)

// Experience levels a student can choose.
const (
	LevelBeginner     = "beginner"
	LevelIntermediate = "intermediate"
	LevelAdvanced     = "advanced"
)

// Levels lists the valid experience levels in display order.
var Levels = []string{LevelBeginner, LevelIntermediate, LevelAdvanced}

// ValidLevel reports whether s is a known experience level.
func ValidLevel(s string) bool {
	for _, l := range Levels {
		if s == l {
			return true
		}
	}
	return false
}

var (
	ErrNotFound           = errors.New("user not found")
	ErrEmailTaken         = errors.New("email already registered")
	ErrInvalidCredentials = errors.New("invalid email or password")
)

// ValidationErrors maps form field names to human-readable problems.
type ValidationErrors map[string]string

func (v ValidationErrors) Error() string {
	keys := make([]string, 0, len(v))
	for k := range v {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+": "+v[k])
	}
	return "validation failed: " + strings.Join(parts, "; ")
}

// User is a registered student.
type User struct {
	ID              int64
	Name            string
	Email           string
	PasswordHash    string
	LanguageID      int64 // 0 when no language is selected
	LanguageName    string
	LanguageSlug    string
	ExperienceLevel string
	CreatedAt       time.Time
}

// FirstName returns the first word of the user's name for friendly greetings.
func (u *User) FirstName() string {
	if f := strings.Fields(u.Name); len(f) > 0 {
		return f[0]
	}
	return u.Name
}

// Store provides database access for users.
type Store struct{ db *sql.DB }

// NewStore returns a Store backed by db.
func NewStore(db *sql.DB) *Store { return &Store{db: db} }

const selectUser = `SELECT u.id, u.name, u.email, u.password_hash, COALESCE(u.language_id, 0),
	COALESCE(l.name, ''), COALESCE(l.slug, ''), u.experience_level, u.created_at
	FROM users u LEFT JOIN programming_languages l ON l.id = u.language_id`

func scanUser(row interface{ Scan(...any) error }) (*User, error) {
	var u User
	var created string
	err := row.Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.LanguageID,
		&u.LanguageName, &u.LanguageSlug, &u.ExperienceLevel, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	u.CreatedAt = database.ParseTime(created)
	return &u, nil
}

// GetByID loads a user by primary key.
func (s *Store) GetByID(ctx context.Context, id int64) (*User, error) {
	return scanUser(s.db.QueryRowContext(ctx, selectUser+` WHERE u.id = ?`, id))
}

// GetByEmail loads a user by email (case-insensitive).
func (s *Store) GetByEmail(ctx context.Context, email string) (*User, error) {
	return scanUser(s.db.QueryRowContext(ctx, selectUser+` WHERE u.email = ?`, email))
}

func (s *Store) create(ctx context.Context, u *User) error {
	now := database.Now()
	var lang any
	if u.LanguageID != 0 {
		lang = u.LanguageID
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO users
		(name, email, password_hash, language_id, experience_level, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		u.Name, u.Email, u.PasswordHash, lang, u.ExperienceLevel, now, now)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed: users.email") {
			return ErrEmailTaken
		}
		return fmt.Errorf("insert user: %w", err)
	}
	u.ID, err = res.LastInsertId()
	return err
}

func (s *Store) languageExists(ctx context.Context, id int64) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM programming_languages WHERE id = ?`, id).Scan(&n)
	return n > 0, err
}

// Service holds account business rules.
type Service struct {
	store *Store
	cost  int
}

// NewService returns a Service. bcryptCost may be 0 to use the default cost.
func NewService(store *Store, bcryptCost int) *Service {
	if bcryptCost == 0 {
		bcryptCost = bcrypt.DefaultCost
	}
	return &Service{store: store, cost: bcryptCost}
}

// Store exposes the underlying store for read-only lookups.
func (s *Service) Store() *Store { return s.store }

// RegisterInput is the data needed to create an account.
type RegisterInput struct {
	Name            string
	Email           string
	Password        string
	LanguageID      int64
	ExperienceLevel string
}

// Register validates input, hashes the password and creates the user.
func (s *Service) Register(ctx context.Context, in RegisterInput) (*User, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	errs := ValidationErrors{}
	validateName(errs, in.Name)
	validateEmail(errs, in.Email)
	validatePassword(errs, "password", in.Password)
	if in.ExperienceLevel == "" {
		in.ExperienceLevel = LevelBeginner
	}
	if !ValidLevel(in.ExperienceLevel) {
		errs["experience_level"] = "Choose a valid experience level."
	}
	if in.LanguageID != 0 {
		ok, err := s.store.languageExists(ctx, in.LanguageID)
		if err != nil {
			return nil, err
		}
		if !ok {
			errs["language_id"] = "Choose a valid language."
		}
	}
	if len(errs) > 0 {
		return nil, errs
	}

	hash, err := s.hash(in.Password)
	if err != nil {
		return nil, err
	}
	u := &User{Name: in.Name, Email: in.Email, PasswordHash: hash,
		LanguageID: in.LanguageID, ExperienceLevel: in.ExperienceLevel}
	if err := s.store.create(ctx, u); err != nil {
		return nil, err
	}
	return s.store.GetByID(ctx, u.ID)
}

// dummyHash is compared against when an email is unknown so that login
// timing does not reveal which emails are registered.
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("timing-equalizer-password"), bcrypt.DefaultCost)

// Authenticate returns the user for valid credentials or ErrInvalidCredentials.
func (s *Service) Authenticate(ctx context.Context, email, password string) (*User, error) {
	u, err := s.store.GetByEmail(ctx, strings.ToLower(strings.TrimSpace(email)))
	if errors.Is(err, ErrNotFound) {
		bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		return nil, ErrInvalidCredentials
	}
	return u, nil
}

// ProfileInput holds editable profile fields.
type ProfileInput struct {
	Name            string
	LanguageID      int64
	ExperienceLevel string
}

// UpdateProfile changes a user's name, language and experience level.
func (s *Service) UpdateProfile(ctx context.Context, userID int64, in ProfileInput) error {
	in.Name = strings.TrimSpace(in.Name)
	errs := ValidationErrors{}
	validateName(errs, in.Name)
	if !ValidLevel(in.ExperienceLevel) {
		errs["experience_level"] = "Choose a valid experience level."
	}
	ok, err := s.store.languageExists(ctx, in.LanguageID)
	if err != nil {
		return err
	}
	if !ok {
		errs["language_id"] = "Choose a valid language."
	}
	if len(errs) > 0 {
		return errs
	}
	res, err := s.store.db.ExecContext(ctx, `UPDATE users SET name = ?, language_id = ?, experience_level = ?, updated_at = ? WHERE id = ?`,
		in.Name, in.LanguageID, in.ExperienceLevel, database.Now(), userID)
	if err != nil {
		return fmt.Errorf("update profile: %w", err)
	}
	return requireOneRow(res)
}

// ChangePassword verifies the current password and stores a new hash.
func (s *Service) ChangePassword(ctx context.Context, userID int64, current, next, confirm string) error {
	u, err := s.store.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	errs := ValidationErrors{}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(current)) != nil {
		errs["current_password"] = "Your current password is incorrect."
	}
	validatePassword(errs, "new_password", next)
	if next != confirm {
		errs["confirm_password"] = "The new passwords do not match."
	}
	if len(errs) > 0 {
		return errs
	}
	hash, err := s.hash(next)
	if err != nil {
		return err
	}
	_, err = s.store.db.ExecContext(ctx, `UPDATE users SET password_hash = ?, updated_at = ? WHERE id = ?`,
		hash, database.Now(), userID)
	return err
}

// Delete permanently removes a user and (via cascading foreign keys) all of
// their learning data, after confirming their password.
func (s *Service) Delete(ctx context.Context, userID int64, password string) error {
	u, err := s.store.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		return ValidationErrors{"delete_password": "Your password is incorrect."}
	}
	res, err := s.store.db.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, userID)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	return requireOneRow(res)
}

func (s *Service) hash(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), s.cost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(b), nil
}

func requireOneRow(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func validateName(errs ValidationErrors, name string) {
	switch n := utf8.RuneCountInString(name); {
	case n == 0:
		errs["name"] = "Please enter your name."
	case n > 80:
		errs["name"] = "Name must be 80 characters or fewer."
	}
}

func validateEmail(errs ValidationErrors, email string) {
	addr, err := mail.ParseAddress(email)
	if email == "" || err != nil || addr.Address != email || len(email) > 254 {
		errs["email"] = "Please enter a valid email address."
	}
}

func validatePassword(errs ValidationErrors, field, pw string) {
	switch {
	case utf8.RuneCountInString(pw) < 8:
		errs[field] = "Password must be at least 8 characters."
	case len(pw) > 72: // bcrypt only uses the first 72 bytes
		errs[field] = "Password must be 72 bytes or fewer."
	}
}

type ctxKey struct{}

// WithUser returns a context carrying the authenticated user.
func WithUser(ctx context.Context, u *User) context.Context {
	return context.WithValue(ctx, ctxKey{}, u)
}

// FromContext returns the authenticated user, or nil if there is none.
func FromContext(ctx context.Context) *User {
	u, _ := ctx.Value(ctxKey{}).(*User)
	return u
}
