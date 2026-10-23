package application_test

import (
	"context"
	"testing"

	"github.com/get-phanuwat-api1/backend-challenge/internal/application"
	"github.com/get-phanuwat-api1/backend-challenge/internal/domain"
	"github.com/get-phanuwat-api1/backend-challenge/internal/mocks"
	"github.com/get-phanuwat-api1/backend-challenge/pkg/logger"
)

func newAuthService() (application.AuthService, *mocks.UserRepositoryFake) {
	repo := mocks.NewUserRepositoryFake()
	hasher := mocks.PasswordHasherFake{}
	log := logger.NewDiscardLogger()
	users := application.NewUserService(repo, hasher, log)
	return application.NewAuthService(users, repo, hasher, mocks.TokenManagerFake{}, log), repo
}

func TestRegisterAndLogin(t *testing.T) {
	auth, _ := newAuthService()
	ctx := context.Background()

	u, err := auth.Register(ctx, application.CreateUserInput{Name: "Alice", Email: "alice@example.com", Password: "password123"})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	tok, err := auth.Login(ctx, " ALICE@example.com ", "password123")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if tok.Value != "token-for-"+u.ID || tok.ExpiresAt.IsZero() {
		t.Fatalf("unexpected token: %+v", tok)
	}

	userID, err := auth.VerifyAccessToken(tok.Value)
	if err != nil || userID != u.ID {
		t.Fatalf("VerifyAccessToken = %q, %v; want %q", userID, err, u.ID)
	}
}

func TestRegister_Validation(t *testing.T) {
	auth, _ := newAuthService()
	_, err := auth.Register(context.Background(), application.CreateUserInput{Name: "A", Email: "bad", Password: "password123"})
	assertCode(t, err, domain.CodeInvalidEmail)
}

func TestLogin_InvalidCredentials(t *testing.T) {
	auth, _ := newAuthService()
	ctx := context.Background()
	if _, err := auth.Register(ctx, application.CreateUserInput{Name: "Alice", Email: "alice@example.com", Password: "password123"}); err != nil {
		t.Fatal(err)
	}

	for _, c := range []struct{ email, password string }{
		{"alice@example.com", "wrong-password"},
		{"nobody@example.com", "password123"},
	} {
		_, err := auth.Login(ctx, c.email, c.password)
		assertCode(t, err, domain.CodeInvalidCredentials)
	}
}

func TestLogin_RepositoryError(t *testing.T) {
	auth, repo := newAuthService()
	repo.Err = errDB
	_, err := auth.Login(context.Background(), "a@b.co", "password123")
	assertCode(t, err, domain.CodeInternal)
}

func TestVerifyAccessToken_Invalid(t *testing.T) {
	auth, _ := newAuthService()
	for _, tok := range []string{"", "garbage", "token-for-"} {
		_, err := auth.VerifyAccessToken(tok)
		assertCode(t, err, domain.CodeUnauthorized)
	}
}
