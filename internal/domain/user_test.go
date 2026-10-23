package domain_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/get-phanuwat-api1/backend-challenge/internal/domain"
)

func ptr[T any](v T) *T { return &v }

func assertCode(t *testing.T, err error, want domain.ErrorCode) {
	t.Helper()
	if got := domain.CodeOf(err); err == nil || got != want {
		t.Fatalf("error = %v (code %s), want code %s", err, got, want)
	}
}

func TestNewUser_Normalizes(t *testing.T) {
	u, err := domain.NewUser("  Alice ", " Alice@Example.COM ", "password123")
	if err != nil {
		t.Fatal(err)
	}
	if u.Name != "Alice" || u.Email != "alice@example.com" || u.PasswordHash != "" {
		t.Fatalf("unexpected user: %+v", u)
	}
}

func TestNewUser_Validation(t *testing.T) {
	tests := []struct {
		name, userName, email, password string
		want                            domain.ErrorCode
	}{
		{"empty name", "  ", "a@b.co", "password123", domain.CodeInvalidName},
		{"long name", strings.Repeat("a", 101), "a@b.co", "password123", domain.CodeInvalidName},
		{"empty email", "A", "", "password123", domain.CodeInvalidEmail},
		{"no at", "A", "alice", "password123", domain.CodeInvalidEmail},
		{"no domain dot", "A", "alice@localhost", "password123", domain.CodeInvalidEmail},
		{"display name", "A", "Alice <alice@example.com>", "password123", domain.CodeInvalidEmail},
		{"too long email", "A", strings.Repeat("a", 250) + "@example.com", "password123", domain.CodeInvalidEmail},
		{"empty password", "A", "a@b.co", "", domain.CodeInvalidPassword},
		{"short password", "A", "a@b.co", "1234567", domain.CodeInvalidPassword},
		{"long password", "A", "a@b.co", strings.Repeat("a", 73), domain.CodeInvalidPassword},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := domain.NewUser(tt.userName, tt.email, tt.password)
			assertCode(t, err, tt.want)
		})
	}
}

func TestNewUser_Boundaries(t *testing.T) {
	if _, err := domain.NewUser(strings.Repeat("ก", 100), "a@b.co", "12345678"); err != nil {
		t.Errorf("100-rune name and 8-char password should be valid: %v", err)
	}
	if _, err := domain.NewUser("A", "a@b.co", strings.Repeat("a", 72)); err != nil {
		t.Errorf("72-byte password should be valid: %v", err)
	}
}

func TestApplyPatch(t *testing.T) {
	base := domain.User{ID: "1", Name: "Alice", Email: "alice@example.com"}

	u := base
	if err := u.ApplyPatch(domain.UserPatch{Name: ptr(" Alicia ")}); err != nil {
		t.Fatal(err)
	}
	if u.Name != "Alicia" || u.Email != base.Email {
		t.Fatalf("name patch: %+v", u)
	}

	u = base
	if err := u.ApplyPatch(domain.UserPatch{Email: ptr("NEW@example.com")}); err != nil || u.Email != "new@example.com" {
		t.Fatalf("email patch: %+v, %v", u, err)
	}

	u = base
	assertCode(t, u.ApplyPatch(domain.UserPatch{}), domain.CodeInvalidRequestBody)

	u = base
	err := u.ApplyPatch(domain.UserPatch{Name: ptr("Bob"), Email: ptr("bad")})
	assertCode(t, err, domain.CodeInvalidEmail)
	if u != base {
		t.Fatalf("user must be unchanged on error: %+v", u)
	}
	if !errors.Is(err, domain.NewError(domain.CodeInvalidEmail, "")) {
		t.Error("errors.Is should match by code")
	}
}
