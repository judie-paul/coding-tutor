package database_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"codingtutor/internal/database"
	"codingtutor/migrations"
)

func TestMigrateIsIdempotentAndSeedsTopics(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(filepath.Join(t.TempDir(), "m.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	first, err := database.Migrate(ctx, db, migrations.FS)
	if err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	if len(first) == 0 {
		t.Fatal("expected migrations to be applied")
	}
	second, err := database.Migrate(ctx, db, migrations.FS)
	if err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	if len(second) != 0 {
		t.Fatalf("expected no migrations on second run, got %v", second)
	}

	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM topics t JOIN programming_languages l ON l.id = t.language_id WHERE l.slug = 'go'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n < 10 {
		t.Fatalf("expected seeded Go topics, got %d", n)
	}
}

func TestForeignKeysEnforced(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(filepath.Join(t.TempDir(), "fk.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := database.Migrate(ctx, db, migrations.FS); err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO sessions (token_hash, user_id, expires_at, created_at, last_seen_at) VALUES ('x', 999, ?, ?, ?)`,
		database.Now(), database.Now(), database.Now())
	if err == nil {
		t.Fatal("expected foreign key violation for unknown user")
	}
}

func TestTimeRoundTrip(t *testing.T) {
	in := time.Date(2026, 3, 4, 5, 6, 7, 0, time.FixedZone("EAT", 3*3600))
	got := database.ParseTime(database.FormatTime(in))
	if !got.Equal(in) || got.Location() != time.UTC {
		t.Fatalf("round trip mismatch: %v vs %v", got, in)
	}
}
