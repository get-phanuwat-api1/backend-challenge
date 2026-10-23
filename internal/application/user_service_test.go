package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/get-phanuwat-api1/backend-challenge/internal/application"
	"github.com/get-phanuwat-api1/backend-challenge/internal/domain"
	"github.com/get-phanuwat-api1/backend-challenge/internal/mocks"
	"github.com/get-phanuwat-api1/backend-challenge/pkg/logger"
)

// Application tests use only fakes for the driven ports: no MongoDB, no
// bcrypt, no JWT. That is the point of the hexagonal core.

var errDB = errors.New("db down")

func ptr[T any](v T) *T { return &v }

func newUserService() (application.UserService, *mocks.UserRepositoryFake) {
	repo := mocks.NewUserRepositoryFake()
	return application.NewUserService(repo, mocks.PasswordHasherFake{}, logger.NewDiscardLogger()), repo
}

func mustCreate(t *testing.T, svc application.UserService, name, email string) *domain.User {
	t.Helper()
	u, err := svc.CreateUser(context.Background(), application.CreateUserInput{Name: name, Email: email, Password: "password123"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	return u
}

func assertCode(t *testing.T, err error, want domain.ErrorCode) {
	t.Helper()
	if got := domain.CodeOf(err); err == nil || got != want {
		t.Fatalf("error = %v (code %s), want code %s", err, got, want)
	}
}

func TestCreateUser(t *testing.T) {
	svc, repo := newUserService()
	before := time.Now().UTC().Add(-time.Second)

	u, err := svc.CreateUser(context.Background(), application.CreateUserInput{
		Name: "  Alice ", Email: " Alice@Example.com ", Password: "password123",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.ID == "" || u.Name != "Alice" || u.Email != "alice@example.com" {
		t.Errorf("unexpected user: %+v", u)
	}
	if u.PasswordHash != "hashed:password123" {
		t.Errorf("password must be hashed through the PasswordHasher port, got %q", u.PasswordHash)
	}
	if u.CreatedAt.Before(before) || u.CreatedAt.Location() != time.UTC {
		t.Errorf("CreatedAt = %v", u.CreatedAt)
	}
	if stored, _ := repo.GetUserByID(context.Background(), u.ID); stored == nil {
		t.Error("user was not persisted")
	}
}

func TestCreateUser_Errors(t *testing.T) {
	tests := []struct {
		name    string
		input   application.CreateUserInput
		repoErr error
		want    domain.ErrorCode
	}{
		{"missing name", application.CreateUserInput{Email: "a@b.co", Password: "password123"}, nil, domain.CodeInvalidName},
		{"bad email", application.CreateUserInput{Name: "A", Email: "nope", Password: "password123"}, nil, domain.CodeInvalidEmail},
		{"short password", application.CreateUserInput{Name: "A", Email: "a@b.co", Password: "short"}, nil, domain.CodeInvalidPassword},
		{"duplicate email", application.CreateUserInput{Name: "A", Email: "TAKEN@example.com", Password: "password123"}, nil, domain.CodeEmailAlreadyExists},
		{"repository failure", application.CreateUserInput{Name: "A", Email: "new@example.com", Password: "password123"}, errDB, domain.CodeInternal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo := newUserService()
			mustCreate(t, svc, "Existing", "taken@example.com")
			repo.Err = tt.repoErr

			_, err := svc.CreateUser(context.Background(), tt.input)
			assertCode(t, err, tt.want)
		})
	}
}

func TestCreateUser_HasherFailure(t *testing.T) {
	svc := application.NewUserService(mocks.NewUserRepositoryFake(), mocks.PasswordHasherFake{Err: errors.New("boom")}, logger.NewDiscardLogger())
	_, err := svc.CreateUser(context.Background(), application.CreateUserInput{Name: "A", Email: "a@b.co", Password: "password123"})
	assertCode(t, err, domain.CodeInternal)
}

func TestGetListAndCountUsers(t *testing.T) {
	svc, _ := newUserService()
	a := mustCreate(t, svc, "Alice", "alice@example.com")
	mustCreate(t, svc, "Bob", "bob@example.com")
	ctx := context.Background()

	got, err := svc.GetUserByID(ctx, a.ID)
	if err != nil || got.Email != "alice@example.com" {
		t.Fatalf("GetUserByID = %+v, %v", got, err)
	}
	_, err = svc.GetUserByID(ctx, "missing")
	assertCode(t, err, domain.CodeUserNotFound)

	if users, err := svc.ListUsers(ctx); err != nil || len(users) != 2 {
		t.Fatalf("ListUsers = %d users, %v", len(users), err)
	}
	if n, err := svc.CountUsers(ctx); err != nil || n != 2 {
		t.Fatalf("CountUsers = %d, %v", n, err)
	}
}

func TestPatchUserByID(t *testing.T) {
	svc, _ := newUserService()
	a := mustCreate(t, svc, "Alice", "alice@example.com")
	b := mustCreate(t, svc, "Bob", "bob@example.com")
	ctx := context.Background()

	t.Run("updates name only", func(t *testing.T) {
		u, err := svc.PatchUserByID(ctx, a.ID, a.ID, domain.UserPatch{Name: ptr(" Alicia ")})
		if err != nil {
			t.Fatal(err)
		}
		if u.Name != "Alicia" || u.Email != "alice@example.com" {
			t.Fatalf("got %+v", u)
		}
	})
	t.Run("updates email and persists", func(t *testing.T) {
		if _, err := svc.PatchUserByID(ctx, a.ID, a.ID, domain.UserPatch{Email: ptr("NEW@example.com")}); err != nil {
			t.Fatal(err)
		}
		stored, _ := svc.GetUserByID(ctx, a.ID)
		if stored.Email != "new@example.com" {
			t.Fatalf("update was not persisted: %+v", stored)
		}
	})
	t.Run("rejects other user's account", func(t *testing.T) {
		_, err := svc.PatchUserByID(ctx, b.ID, a.ID, domain.UserPatch{Name: ptr("X")})
		assertCode(t, err, domain.CodeForbidden)
	})
	t.Run("rejects missing actor", func(t *testing.T) {
		_, err := svc.PatchUserByID(ctx, "", a.ID, domain.UserPatch{Name: ptr("X")})
		assertCode(t, err, domain.CodeForbidden)
	})
	t.Run("rejects empty patch", func(t *testing.T) {
		_, err := svc.PatchUserByID(ctx, a.ID, a.ID, domain.UserPatch{})
		assertCode(t, err, domain.CodeInvalidRequestBody)
	})
	t.Run("rejects invalid email", func(t *testing.T) {
		_, err := svc.PatchUserByID(ctx, a.ID, a.ID, domain.UserPatch{Email: ptr("bad")})
		assertCode(t, err, domain.CodeInvalidEmail)
	})
	t.Run("rejects duplicate email", func(t *testing.T) {
		_, err := svc.PatchUserByID(ctx, a.ID, a.ID, domain.UserPatch{Email: ptr("bob@example.com")})
		assertCode(t, err, domain.CodeEmailAlreadyExists)
	})
	t.Run("not found", func(t *testing.T) {
		_, err := svc.PatchUserByID(ctx, "ghost", "ghost", domain.UserPatch{Name: ptr("X")})
		assertCode(t, err, domain.CodeUserNotFound)
	})
}

func TestDeleteUserByID(t *testing.T) {
	svc, _ := newUserService()
	a := mustCreate(t, svc, "Alice", "alice@example.com")
	b := mustCreate(t, svc, "Bob", "bob@example.com")
	ctx := context.Background()

	assertCode(t, svc.DeleteUserByID(ctx, b.ID, a.ID), domain.CodeForbidden)

	if err := svc.DeleteUserByID(ctx, a.ID, a.ID); err != nil {
		t.Fatalf("DeleteUserByID: %v", err)
	}
	_, err := svc.GetUserByID(ctx, a.ID)
	assertCode(t, err, domain.CodeUserNotFound)
	assertCode(t, svc.DeleteUserByID(ctx, a.ID, a.ID), domain.CodeUserNotFound)
}
