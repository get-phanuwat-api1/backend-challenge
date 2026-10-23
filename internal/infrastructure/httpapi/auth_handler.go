package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/get-phanuwat-api1/backend-challenge/internal/application"
)

// AuthHandler exposes registration and login over HTTP.
type AuthHandler struct {
	authService application.AuthService
	logger      *slog.Logger
}

func NewAuthHandler(authService application.AuthService, logger *slog.Logger) *AuthHandler {
	return &AuthHandler{authService: authService, logger: logger}
}

// Register godoc
//
//	@ID				register
//	@Summary		Register a new user
//	@Description	Public endpoint. Email is normalized (trimmed, lowercased) and must be unique.
//	@Description	Password must be 8–72 bytes and is stored as a bcrypt hash.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		CreateUserRequest	true	"User to register"
//	@Success		201		{object}	UserResponse
//	@Failure		400		{object}	ErrorResponse	"INVALID_REQUEST_BODY | INVALID_NAME | INVALID_EMAIL | INVALID_PASSWORD"
//	@Failure		409		{object}	ErrorResponse	"EMAIL_ALREADY_EXISTS"
//	@Failure		500		{object}	ErrorResponse	"INTERNAL_SERVER_ERROR"
//	@Router			/api/v1/auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	var req CreateUserRequest
	if err := decodeJSONBody(c, &req); err != nil {
		writeError(c, h.logger, err)
		return
	}
	u, err := h.authService.Register(c.Request.Context(), application.CreateUserInput(req))
	if err != nil {
		writeError(c, h.logger, err)
		return
	}
	c.Header("Location", "/api/v1/users/"+u.ID)
	c.JSON(http.StatusCreated, UserResponse{Successful: true, Data: NewUserData(u)})
}

// Login godoc
//
//	@ID				login
//	@Summary		Log in and get a JWT
//	@Description	Returns an HS256-signed access token. Send it as `Authorization: Bearer <token>`.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		LoginRequest	true	"Credentials"
//	@Success		200		{object}	LoginResponse
//	@Failure		400		{object}	ErrorResponse	"INVALID_REQUEST_BODY"
//	@Failure		401		{object}	ErrorResponse	"INVALID_CREDENTIALS"
//	@Failure		500		{object}	ErrorResponse	"INTERNAL_SERVER_ERROR"
//	@Router			/api/v1/auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := decodeJSONBody(c, &req); err != nil {
		writeError(c, h.logger, err)
		return
	}
	tok, err := h.authService.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		writeError(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, LoginResponse{Successful: true, Data: NewTokenData(tok)})
}
