// Package grpcapi is the gRPC adapter: it exposes CreateUser and GetUser by
// calling the same application ports (use cases) as the HTTP adapter.
package grpcapi

import (
	"context"
	"log/slog"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/get-phanuwat-api1/backend-challenge/internal/application"
	"github.com/get-phanuwat-api1/backend-challenge/internal/domain"
	userv1 "github.com/get-phanuwat-api1/backend-challenge/proto/user/v1"
)

// NewServer returns a gRPC server with request ID, logging and JWT auth
// interceptors and the UserService registered.
func NewServer(userService application.UserService, authService application.AuthService, logger *slog.Logger) *grpc.Server {
	srv := grpc.NewServer(grpc.ChainUnaryInterceptor(
		RequestIDInterceptor(),
		LoggingInterceptor(logger),
		AuthInterceptor(authService),
	))
	userv1.RegisterUserServiceServer(srv, &userServer{userService: userService, logger: logger})
	reflection.Register(srv)
	return srv
}

type userServer struct {
	userv1.UnimplementedUserServiceServer
	userService application.UserService
	logger      *slog.Logger
}

func (s *userServer) CreateUser(ctx context.Context, req *userv1.CreateUserRequest) (*userv1.CreateUserResponse, error) {
	u, err := s.userService.CreateUser(ctx, application.CreateUserInput{
		Name:     req.GetName(),
		Email:    req.GetEmail(),
		Password: req.GetPassword(),
	})
	if err != nil {
		return nil, s.toStatus(ctx, err)
	}
	return &userv1.CreateUserResponse{User: toProto(u)}, nil
}

func (s *userServer) GetUser(ctx context.Context, req *userv1.GetUserRequest) (*userv1.GetUserResponse, error) {
	u, err := s.userService.GetUserByID(ctx, req.GetId())
	if err != nil {
		return nil, s.toStatus(ctx, err)
	}
	return &userv1.GetUserResponse{User: toProto(u)}, nil
}

func toProto(u *domain.User) *userv1.User {
	return &userv1.User{
		Id:        u.ID,
		Name:      u.Name,
		Email:     u.Email,
		CreatedAt: timestamppb.New(u.CreatedAt),
	}
}

// toStatus maps a domain error to a gRPC status whose message is the error code.
func (s *userServer) toStatus(ctx context.Context, err error) error {
	code := domain.CodeOf(err)
	switch code {
	case domain.CodeUserNotFound:
		return status.Error(codes.NotFound, string(code))
	case domain.CodeEmailAlreadyExists:
		return status.Error(codes.AlreadyExists, string(code))
	case domain.CodeForbidden:
		return status.Error(codes.PermissionDenied, string(code))
	case domain.CodeUnauthorized, domain.CodeInvalidCredentials:
		return status.Error(codes.Unauthenticated, string(code))
	case domain.CodeInternal:
		s.logger.ErrorContext(ctx, "unexpected grpc error", slog.Any("error", err))
		return status.Error(codes.Internal, string(code))
	default:
		return status.Error(codes.InvalidArgument, string(code))
	}
}
