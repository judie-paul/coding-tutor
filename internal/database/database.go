// Package database opens the SQLite database and applies schema migrations.
package database

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"

	_ "modernc.org/sqlite" // pure-Go SQLite driver (no cgo/gcc needed)
)

// TimeFormat is the layout used for every timestamp stored in the database.
const TimeFormat = time.RFC3339

// Now returns the current UTC time formatted for storage.
func Now() string { return FormatTime(time.Now()) }

// FormatTime formats t in UTC for storage.
func FormatTime(t time.Time) string { return t.UTC().Format(TimeFormat) }

// ParseTime parses a stored timestamp. Invalid values yield the zero time.
func ParseTime(s string) time.Time {
	t, err := time.Parse(TimeFormat, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

// DBTX is satisfied by both *sql.DB and *sql.Tx so store methods can run
// inside or outside a transaction.
type DBTX interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Open opens (creating if needed) the SQLite database at path with foreign
// keys enforced, WAL journaling and a busy timeout.
func Open(path string) (*sql.DB, error) {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, fmt.Errorf("create database directory: %w", err)
		}
	}
	dsn := "file:" + filepath.ToSlash(path) +
		"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)"

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("connect database: %w", err)
	}
	var fk int
	if err := db.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&fk); err != nil || fk != 1 {
		db.Close()
		return nil, fmt.Errorf("foreign keys could not be enabled")
	}
	return db, nil
}

// Migrate applies every *.sql file in migrations that has not been applied
// yet, in filename order, each inside its own transaction. It returns the
// names of newly applied migrations.
func Migrate(ctx context.Context, db *sql.DB, migrations fs.FS) ([]string, error) {
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version    TEXT PRIMARY KEY,
		applied_at TEXT NOT NULL
	)`); err != nil {
		return nil, fmt.Errorf("create schema_migrations: %w", err)
	}

	names, err := fs.Glob(migrations, "*.sql")
	if err != nil {
		return nil, err
	}
	sort.Strings(names)

	var applied []string
	for _, name := range names {
		var exists int
		err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations WHERE version = ?`, name).Scan(&exists)
		if err != nil {
			return applied, fmt.Errorf("check migration %s: %w", name, err)
		}
		if exists > 0 {
			continue
		}
		body, err := fs.ReadFile(migrations, name)
		if err != nil {
			return applied, err
		}
		if err := applyMigration(ctx, db, name, string(body)); err != nil {
			return applied, err
		}
		applied = append(applied, name)
	}
	return applied, nil
}

func applyMigration(ctx context.Context, db *sql.DB, name, body string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, body); err != nil {
		return fmt.Errorf("apply migration %s: %w", name, err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`, name, Now()); err != nil {
		return fmt.Errorf("record migration %s: %w", name, err)
	}
	return tx.Commit()
}

// InTx runs fn inside a transaction, committing on success and rolling back
// on error.
func InTx(ctx context.Context, db *sql.DB, fn func(tx *sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}
