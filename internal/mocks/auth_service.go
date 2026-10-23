package mocks

import (
	"context"

	"github.com/get-phanuwat-api1/backend-challenge/internal/application"
	"github.com/get-phanuwat-api1/backend-challenge/internal/domain"
)

// AuthService is a mock application.AuthService. Set the *Func field for each method a
// test expects to be called; calling a method without one fails loudly.
type AuthService struct {
	RegisterFunc          func(ctx context.Context, input application.CreateUserInput) (*domain.User, error)
	LoginFunc             func(ctx context.Context, email, password string) (domain.AccessToken, error)
	VerifyAccessTokenFunc func(token string) (string, error)
}

var _ application.AuthService = (*AuthService)(nil)

func (m *AuthService) Register(ctx context.Context, input application.CreateUserInput) (*domain.User, error) {
	if m.RegisterFunc == nil {
		panic("mocks.AuthService: unexpected call to Register")
	}
	return m.RegisterFunc(ctx, input)
}

func (m *AuthService) Login(ctx context.Context, email, password string) (domain.AccessToken, error) {
	if m.LoginFunc == nil {
		panic("mocks.AuthService: unexpected call to Login")
	}
	return m.LoginFunc(ctx, email, password)
}

func (m *AuthService) VerifyAccessToken(token string) (string, error) {
	if m.VerifyAccessTokenFunc == nil {
		panic("mocks.AuthService: unexpected call to VerifyAccessToken")
	}
	return m.VerifyAccessTokenFunc(token)
}
