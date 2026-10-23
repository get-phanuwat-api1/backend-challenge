package application

import (
	"context"
	"errors"
	"log/slog"

	"github.com/get-phanuwat-api1/backend-challenge/internal/domain"
)

type authService struct {
	userService UserService
	userRepo    UserRepository
	hasher      PasswordHasher
	tokens      TokenManager
	logger      *slog.Logger
}

// NewAuthService wires the auth use cases to their driven ports.
func NewAuthService(
	userService UserService,
	userRepo UserRepository,
	hasher PasswordHasher,
	tokens TokenManager,
	logger *slog.Logger,
) AuthService {
	return &authService{userService: userService, userRepo: userRepo, hasher: hasher, tokens: tokens, logger: logger}
}

func (s *authService) Register(ctx context.Context, input CreateUserInput) (*domain.User, error) {
	return s.userService.CreateUser(ctx, input)
}

func (s *authService) Login(ctx context.Context, email, password string) (domain.AccessToken, error) {
	u, err := s.userRepo.GetUserByEmail(ctx, domain.NormalizeEmail(email))
	if errors.Is(err, domain.ErrUserNotFound) {
		return domain.AccessToken{}, domain.ErrInvalidCredentials
	}
	if err != nil {
		return domain.AccessToken{}, err
	}
	if err := s.hasher.Compare(u.PasswordHash, password); err != nil {
		return domain.AccessToken{}, domain.ErrInvalidCredentials
	}

	tok, err := s.tokens.Generate(u.ID)
	if err != nil {
		return domain.AccessToken{}, err
	}
	s.logger.InfoContext(ctx, "user logged in", slog.String("login_user_id", u.ID))
	return tok, nil
}

func (s *authService) VerifyAccessToken(token string) (string, error) {
	userID, err := s.tokens.Validate(token)
	if err != nil || userID == "" {
		return "", domain.ErrUnauthorized
	}
	return userID, nil
}
