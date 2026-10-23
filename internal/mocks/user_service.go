package mocks

import (
	"context"

	"github.com/get-phanuwat-api1/backend-challenge/internal/application"
	"github.com/get-phanuwat-api1/backend-challenge/internal/domain"
)

// UserService is a mock application.UserService. Set the *Func field for each method a
// test expects to be called; calling a method without one fails loudly.
type UserService struct {
	CreateUserFunc     func(ctx context.Context, input application.CreateUserInput) (*domain.User, error)
	GetUserByIDFunc    func(ctx context.Context, id string) (*domain.User, error)
	ListUsersFunc      func(ctx context.Context) ([]domain.User, error)
	CountUsersFunc     func(ctx context.Context) (int64, error)
	PatchUserByIDFunc  func(ctx context.Context, actorID, id string, patch domain.UserPatch) (*domain.User, error)
	DeleteUserByIDFunc func(ctx context.Context, actorID, id string) error
}

var _ application.UserService = (*UserService)(nil)

func (m *UserService) CreateUser(ctx context.Context, input application.CreateUserInput) (*domain.User, error) {
	if m.CreateUserFunc == nil {
		panic("mocks.UserService: unexpected call to CreateUser")
	}
	return m.CreateUserFunc(ctx, input)
}

func (m *UserService) GetUserByID(ctx context.Context, id string) (*domain.User, error) {
	if m.GetUserByIDFunc == nil {
		panic("mocks.UserService: unexpected call to GetUserByID")
	}
	return m.GetUserByIDFunc(ctx, id)
}

func (m *UserService) ListUsers(ctx context.Context) ([]domain.User, error) {
	if m.ListUsersFunc == nil {
		panic("mocks.UserService: unexpected call to ListUsers")
	}
	return m.ListUsersFunc(ctx)
}

func (m *UserService) CountUsers(ctx context.Context) (int64, error) {
	if m.CountUsersFunc == nil {
		panic("mocks.UserService: unexpected call to CountUsers")
	}
	return m.CountUsersFunc(ctx)
}

func (m *UserService) PatchUserByID(ctx context.Context, actorID, id string, patch domain.UserPatch) (*domain.User, error) {
	if m.PatchUserByIDFunc == nil {
		panic("mocks.UserService: unexpected call to PatchUserByID")
	}
	return m.PatchUserByIDFunc(ctx, actorID, id, patch)
}

func (m *UserService) DeleteUserByID(ctx context.Context, actorID, id string) error {
	if m.DeleteUserByIDFunc == nil {
		panic("mocks.UserService: unexpected call to DeleteUserByID")
	}
	return m.DeleteUserByIDFunc(ctx, actorID, id)
}
