package application

import (
	"context"

	"github.com/get-phanuwat-api1/backend-challenge/internal/domain"
)

// This file holds every port of the hexagon: the interfaces through which the
// application talks to the outside world.

// ─── Driving ports ──────────────────────────────────────────────────────────
// What the application offers. Called by infrastructure/httpapi, grpcapi and
// scheduler; implemented by the services in this package.

// CreateUserInput is the input for creating (registering) a user.
type CreateUserInput struct {
	Name     string
	Email    string
	Password string
}

// UserCounter reports how many users exist.
type UserCounter interface {
	CountUsers(ctx context.Context) (int64, error)
}

// UserService defines the user use cases.
type UserService interface {
	UserCounter
	// CreateUser validates the input, hashes the password and stores a new user.
	CreateUser(ctx context.Context, input CreateUserInput) (*domain.User, error)
	GetUserByID(ctx context.Context, id string) (*domain.User, error)
	ListUsers(ctx context.Context) ([]domain.User, error)
	// PatchUserByID overwrites only the provided fields. actorID is the
	// authenticated caller; only the account owner may update it.
	PatchUserByID(ctx context.Context, actorID, id string, patch domain.UserPatch) (*domain.User, error)
	// DeleteUserByID removes a user. Only the account owner may delete it.
	DeleteUserByID(ctx context.Context, actorID, id string) error
}

// AuthService defines registration and authentication use cases.
type AuthService interface {
	Register(ctx context.Context, input CreateUserInput) (*domain.User, error)
	// Login verifies credentials and returns an access token. Unknown emails
	// and wrong passwords both yield domain.ErrInvalidCredentials.
	Login(ctx context.Context, email, password string) (domain.AccessToken, error)
	// VerifyAccessToken returns the user ID the token was issued for, or
	// domain.ErrUnauthorized.
	VerifyAccessToken(token string) (string, error)
}

// ─── Driven ports ───────────────────────────────────────────────────────────
// What the application needs. Implemented by infrastructure/mongodb and
// infrastructure/security; the application never sees those packages.

// UserRepository persists users. Implementations return
// domain.ErrInvalidUserID for a malformed ID, domain.ErrUserNotFound when a
// user does not exist and domain.ErrEmailAlreadyExists for a taken email.
type UserRepository interface {
	// CreateUser inserts u and fills its ID.
	CreateUser(ctx context.Context, u *domain.User) error
	GetUserByID(ctx context.Context, id string) (*domain.User, error)
	// GetUserByEmail looks a user up by its (normalized) email.
	GetUserByEmail(ctx context.Context, email string) (*domain.User, error)
	// ListUsers returns all users ordered by creation.
	ListUsers(ctx context.Context) ([]domain.User, error)
	// UpdateUser overwrites the name and email of an existing user.
	UpdateUser(ctx context.Context, u *domain.User) error
	DeleteUserByID(ctx context.Context, id string) error
	CountUsers(ctx context.Context) (int64, error)
}

// PasswordHasher hashes and verifies passwords.
type PasswordHasher interface {
	Hash(password string) (string, error)
	// Compare returns a non-nil error if password does not match hash.
	Compare(hash, password string) error
}

// TokenManager issues and validates access tokens.
type TokenManager interface {
	// Generate issues a token whose subject is userID.
	Generate(userID string) (domain.AccessToken, error)
	// Validate returns the user ID the token was issued for, or an error.
	Validate(token string) (string, error)
}
