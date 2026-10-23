// Package application is the Application layer of the hexagonal architecture:
// the use cases (services) and the ports (interfaces) they expose and need.
// It depends only on the domain; it knows nothing about HTTP, gRPC,
// MongoDB, bcrypt or JWT.
package application

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/get-phanuwat-api1/backend-challenge/internal/domain"
)

type userService struct {
	userRepo UserRepository
	hasher   PasswordHasher
	logger   *slog.Logger
}

// NewUserService wires the use cases to their driven ports.
func NewUserService(userRepo UserRepository, hasher PasswordHasher, logger *slog.Logger) UserService {
	return &userService{userRepo: userRepo, hasher: hasher, logger: logger}
}

func (s *userService) CreateUser(ctx context.Context, input CreateUserInput) (*domain.User, error) {
	u, err := domain.NewUser(input.Name, input.Email, input.Password)
	if err != nil {
		return nil, err
	}
	if u.PasswordHash, err = s.hasher.Hash(input.Password); err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}
	u.CreatedAt = time.Now().UTC().Truncate(time.Millisecond) // MongoDB stores millisecond precision

	if err := s.userRepo.CreateUser(ctx, u); err != nil {
		return nil, err
	}
	s.logger.InfoContext(ctx, "user created", slog.String("created_user_id", u.ID))
	return u, nil
}

func (s *userService) GetUserByID(ctx context.Context, id string) (*domain.User, error) {
	return s.userRepo.GetUserByID(ctx, id)
}

func (s *userService) ListUsers(ctx context.Context) ([]domain.User, error) {
	return s.userRepo.ListUsers(ctx)
}

func (s *userService) CountUsers(ctx context.Context) (int64, error) {
	return s.userRepo.CountUsers(ctx)
}

func (s *userService) PatchUserByID(ctx context.Context, actorID, id string, patch domain.UserPatch) (*domain.User, error) {
	if err := ensureOwner(actorID, id); err != nil {
		return nil, err
	}
	u, err := s.userRepo.GetUserByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := u.ApplyPatch(patch); err != nil {
		return nil, err
	}
	if err := s.userRepo.UpdateUser(ctx, u); err != nil {
		return nil, err
	}
	s.logger.InfoContext(ctx, "user updated", slog.String("updated_user_id", id))
	return u, nil
}

func (s *userService) DeleteUserByID(ctx context.Context, actorID, id string) error {
	if err := ensureOwner(actorID, id); err != nil {
		return err
	}
	if err := s.userRepo.DeleteUserByID(ctx, id); err != nil {
		return err
	}
	s.logger.InfoContext(ctx, "user deleted", slog.String("deleted_user_id", id))
	return nil
}

// ensureOwner allows users to modify only their own account.
func ensureOwner(actorID, id string) error {
	if actorID == "" || actorID != id {
		return domain.ErrForbidden
	}
	return nil
}
