// Package testutil provides helpers shared by tests across packages.
package testutil

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"codingtutor/internal/database"
	"codingtutor/migrations"
)

// NewDB returns a fully migrated SQLite database in a temporary directory.
func NewDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := database.Migrate(context.Background(), db, migrations.FS); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}
