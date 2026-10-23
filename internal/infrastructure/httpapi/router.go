// Package httpapi is the HTTP (REST) adapter, built with Gin: routes,
// middleware, handlers and DTOs. It calls the application through its ports.
package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "github.com/get-phanuwat-api1/backend-challenge/docs" // generated swagger "spec"
	"github.com/get-phanuwat-api1/backend-challenge/internal/application"
)

// NewRouter builds the gin engine with middleware and all routes registered.
func NewRouter(
	logger *slog.Logger,
	authService application.AuthService,
	authHandler *AuthHandler,
	userHandler *UserHandler,
) *gin.Engine {
	r := gin.New()
	// Order matters: request context first, recovery innermost so the access log sees the 500.
	r.Use(
		RequestID(),
		AccessLog(logger, "/health", "/api-docs"),
		Recovery(logger),
	)

	r.GET("/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })

	v1 := r.Group("/api/v1")

	auth := v1.Group("/auth")
	auth.POST("/register", authHandler.Register)
	auth.POST("/login", authHandler.Login)

	users := v1.Group("/users", Auth(authService))
	users.POST("", userHandler.CreateUser)
	users.GET("", userHandler.ListUsers)
	users.GET("/me", userHandler.GetMe)
	users.GET("/:id", userHandler.GetUserByID)
	users.PATCH("/:id", userHandler.PatchUserByID)
	users.DELETE("/:id", userHandler.DeleteUserByID)

	r.GET("/api-docs", func(c *gin.Context) { c.Redirect(http.StatusMovedPermanently, "/api-docs/index.html") })
	r.GET("/api-docs/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	return r
}
