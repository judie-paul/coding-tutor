// Package topics provides programming languages and their learning topics.
package topics

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

// ErrNotFound is returned when a language or topic does not exist.
var ErrNotFound = errors.New("not found")

// Language is a programming language students can learn.
type Language struct {
	ID          int64
	Slug        string
	Name        string
	Description string
}

// Topic is one concept in a language's learning path.
type Topic struct {
	ID         int64
	LanguageID int64
	Language   string
	Slug       string
	Title      string
	Summary    string
	KeyPoints  []string
	Difficulty string
	Position   int
}

// Store reads languages and topics.
type Store struct{ db *sql.DB }

// NewStore returns a Store backed by db.
func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// Languages lists all languages ordered by name.
func (s *Store) Languages(ctx context.Context) ([]Language, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, slug, name, description FROM programming_languages ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Language
	for rows.Next() {
		var l Language
		if err := rows.Scan(&l.ID, &l.Slug, &l.Name, &l.Description); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// DefaultLanguageID returns the ID of the first language (Go in this version).
func (s *Store) DefaultLanguageID(ctx context.Context) (int64, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `SELECT id FROM programming_languages ORDER BY id LIMIT 1`).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	return id, err
}

const selectTopic = `SELECT t.id, t.language_id, l.name, t.slug, t.title, t.summary, t.key_points, t.difficulty, t.position
	FROM topics t JOIN programming_languages l ON l.id = t.language_id`

func scanTopic(row interface{ Scan(...any) error }) (Topic, error) {
	var t Topic
	var points string
	err := row.Scan(&t.ID, &t.LanguageID, &t.Language, &t.Slug, &t.Title, &t.Summary, &points, &t.Difficulty, &t.Position)
	if err != nil {
		return t, err
	}
	for _, p := range strings.Split(points, "\n") {
		if p = strings.TrimSpace(p); p != "" {
			t.KeyPoints = append(t.KeyPoints, p)
		}
	}
	return t, nil
}

// List returns all topics for a language in learning-path order.
func (s *Store) List(ctx context.Context, languageID int64) ([]Topic, error) {
	rows, err := s.db.QueryContext(ctx, selectTopic+` WHERE t.language_id = ? ORDER BY t.position`, languageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Topic
	for rows.Next() {
		t, err := scanTopic(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// Get returns a topic by ID.
func (s *Store) Get(ctx context.Context, id int64) (Topic, error) {
	t, err := scanTopic(s.db.QueryRowContext(ctx, selectTopic+` WHERE t.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return t, ErrNotFound
	}
	return t, err
}

// GetBySlug returns a language's topic by slug.
func (s *Store) GetBySlug(ctx context.Context, languageID int64, slug string) (Topic, error) {
	t, err := scanTopic(s.db.QueryRowContext(ctx, selectTopic+` WHERE t.language_id = ? AND t.slug = ?`, languageID, slug))
	if errors.Is(err, sql.ErrNoRows) {
		return t, ErrNotFound
	}
	return t, err
}
