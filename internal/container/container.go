// Package container is the composition root: the only place that knows every
// concrete type. It plugs infrastructure (MongoDB, bcrypt, JWT) into the
// application, then plugs the application into HTTP and gRPC.
package container

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"google.golang.org/grpc"

	"github.com/get-phanuwat-api1/backend-challenge/internal/application"
	"github.com/get-phanuwat-api1/backend-challenge/internal/infrastructure/grpcapi"
	"github.com/get-phanuwat-api1/backend-challenge/internal/infrastructure/httpapi"
	"github.com/get-phanuwat-api1/backend-challenge/internal/infrastructure/mongodb"
)

// Container holds the wired application graph.
type Container struct {
	UserService application.UserService
	AuthService application.AuthService
	Router      *gin.Engine
	GRPCServer  *grpc.Server
}

// NewContainer wires all dependencies.
func NewContainer(db *mongo.Database, hasher application.PasswordHasher, tokens application.TokenManager, logger *slog.Logger) *Container {
	// Infrastructure the application needs (implements its driven ports)
	userRepo := mongodb.NewUserRepository(db)

	// Application (use cases)
	userSvc := application.NewUserService(userRepo, hasher, logger.With("layer", "application"))
	authSvc := application.NewAuthService(userSvc, userRepo, hasher, tokens, logger.With("layer", "application"))

	// Infrastructure that calls the application (uses its driving ports)
	authHandler := httpapi.NewAuthHandler(authSvc, logger.With("layer", "http"))
	userHandler := httpapi.NewUserHandler(userSvc, logger.With("layer", "http"))

	return &Container{
		UserService: userSvc,
		AuthService: authSvc,
		Router:      httpapi.NewRouter(logger.With("layer", "http"), authSvc, authHandler, userHandler),
		GRPCServer:  grpcapi.NewServer(userSvc, authSvc, logger.With("layer", "grpc")),
	}
}
