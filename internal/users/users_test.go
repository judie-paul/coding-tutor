package users_test

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"codingtutor/internal/testutil"
	"codingtutor/internal/users"
)

func newService(t *testing.T) *users.Service {
	t.Helper()
	db := testutil.NewDB(t)
	return users.NewService(users.NewStore(db), bcrypt.MinCost)
}

func TestRegisterAndAuthenticate(t *testing.T) {
	ctx := context.Background()
	svc := newService(t)

	u, err := svc.Register(ctx, users.RegisterInput{Name: "Ada Lovelace", Email: " Ada@Example.com ", Password: "correct horse", LanguageID: 1})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if u.Email != "ada@example.com" {
		t.Errorf("email not normalized: %q", u.Email)
	}
	if u.PasswordHash == "correct horse" || u.PasswordHash == "" {
		t.Error("password must be stored hashed")
	}
	if u.LanguageSlug != "go" || u.ExperienceLevel != users.LevelBeginner {
		t.Errorf("unexpected defaults: lang=%q level=%q", u.LanguageSlug, u.ExperienceLevel)
	}

	if _, err := svc.Authenticate(ctx, "ADA@example.com", "correct horse"); err != nil {
		t.Errorf("authenticate with valid credentials: %v", err)
	}
	if _, err := svc.Authenticate(ctx, "ada@example.com", "wrong password"); !errors.Is(err, users.ErrInvalidCredentials) {
		t.Errorf("wrong password: got %v", err)
	}
	if _, err := svc.Authenticate(ctx, "nobody@example.com", "whatever1"); !errors.Is(err, users.ErrInvalidCredentials) {
		t.Errorf("unknown email: got %v", err)
	}
}

func TestRegisterValidation(t *testing.T) {
	ctx := context.Background()
	svc := newService(t)

	tests := []struct {
		name  string
		in    users.RegisterInput
		field string
	}{
		{"missing name", users.RegisterInput{Email: "a@b.co", Password: "password1"}, "name"},
		{"bad email", users.RegisterInput{Name: "A", Email: "not-an-email", Password: "password1"}, "email"},
		{"short password", users.RegisterInput{Name: "A", Email: "a@b.co", Password: "short"}, "password"},
		{"bad level", users.RegisterInput{Name: "A", Email: "a@b.co", Password: "password1", ExperienceLevel: "guru"}, "experience_level"},
		{"bad language", users.RegisterInput{Name: "A", Email: "a@b.co", Password: "password1", LanguageID: 999}, "language_id"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Register(ctx, tt.in)
			var verrs users.ValidationErrors
			if !errors.As(err, &verrs) {
				t.Fatalf("expected validation error, got %v", err)
			}
			if _, ok := verrs[tt.field]; !ok {
				t.Errorf("expected error on %q, got %v", tt.field, verrs)
			}
		})
	}
}

func TestRegisterDuplicateEmail(t *testing.T) {
	ctx := context.Background()
	svc := newService(t)
	in := users.RegisterInput{Name: "A", Email: "dup@example.com", Password: "password1"}
	if _, err := svc.Register(ctx, in); err != nil {
		t.Fatal(err)
	}
	in.Email = "DUP@example.com"
	if _, err := svc.Register(ctx, in); !errors.Is(err, users.ErrEmailTaken) {
		t.Fatalf("expected ErrEmailTaken, got %v", err)
	}
}

func TestChangePasswordAndDelete(t *testing.T) {
	ctx := context.Background()
	svc := newService(t)
	u, err := svc.Register(ctx, users.RegisterInput{Name: "B", Email: "b@example.com", Password: "password1"})
	if err != nil {
		t.Fatal(err)
	}

	var verrs users.ValidationErrors
	if err := svc.ChangePassword(ctx, u.ID, "wrong", "newpassword", "newpassword"); !errors.As(err, &verrs) || verrs["current_password"] == "" {
		t.Fatalf("expected current_password error, got %v", err)
	}
	if err := svc.ChangePassword(ctx, u.ID, "password1", "newpassword", "different"); !errors.As(err, &verrs) || verrs["confirm_password"] == "" {
		t.Fatalf("expected confirm_password error, got %v", err)
	}
	if err := svc.ChangePassword(ctx, u.ID, "password1", "newpassword", "newpassword"); err != nil {
		t.Fatalf("change password: %v", err)
	}
	if _, err := svc.Authenticate(ctx, "b@example.com", "newpassword"); err != nil {
		t.Fatalf("login with new password: %v", err)
	}

	if err := svc.Delete(ctx, u.ID, "wrong"); err == nil {
		t.Fatal("delete with wrong password should fail")
	}
	if err := svc.Delete(ctx, u.ID, "newpassword"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := svc.Store().GetByID(ctx, u.ID); !errors.Is(err, users.ErrNotFound) {
		t.Fatalf("user should be gone, got %v", err)
	}
}
