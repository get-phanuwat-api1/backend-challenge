package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/get-phanuwat-api1/backend-challenge/internal/application"
	"github.com/get-phanuwat-api1/backend-challenge/pkg/requestctx"
)

// UserHandler exposes user operations over HTTP. All routes require a JWT.
type UserHandler struct {
	userService application.UserService
	logger      *slog.Logger
}

func NewUserHandler(userService application.UserService, logger *slog.Logger) *UserHandler {
	return &UserHandler{userService: userService, logger: logger}
}

// CreateUser godoc
//
//	@ID				createUser
//	@Summary		Create a new user
//	@Description	Same rules as register, but requires an authenticated caller.
//	@Tags			user
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			body	body		CreateUserRequest	true	"User to create"
//	@Success		201		{object}	UserResponse
//	@Failure		400		{object}	ErrorResponse	"INVALID_REQUEST_BODY | INVALID_NAME | INVALID_EMAIL | INVALID_PASSWORD"
//	@Failure		401		{object}	ErrorResponse	"UNAUTHORIZED"
//	@Failure		409		{object}	ErrorResponse	"EMAIL_ALREADY_EXISTS"
//	@Failure		500		{object}	ErrorResponse	"INTERNAL_SERVER_ERROR"
//	@Router			/api/v1/users [post]
func (h *UserHandler) CreateUser(c *gin.Context) {
	var req CreateUserRequest
	if err := decodeJSONBody(c, &req); err != nil {
		writeError(c, h.logger, err)
		return
	}
	u, err := h.userService.CreateUser(c.Request.Context(), application.CreateUserInput(req))
	if err != nil {
		writeError(c, h.logger, err)
		return
	}
	c.Header("Location", "/api/v1/users/"+u.ID)
	c.JSON(http.StatusCreated, UserResponse{Successful: true, Data: NewUserData(u)})
}

// ListUsers godoc
//
//	@ID			listUsers
//	@Summary	List all users
//	@Tags		user
//	@Produce	json
//	@Security	BearerAuth
//	@Success	200	{object}	ListUsersResponse
//	@Failure	401	{object}	ErrorResponse	"UNAUTHORIZED"
//	@Failure	500	{object}	ErrorResponse	"INTERNAL_SERVER_ERROR"
//	@Router		/api/v1/users [get]
func (h *UserHandler) ListUsers(c *gin.Context) {
	users, err := h.userService.ListUsers(c.Request.Context())
	if err != nil {
		writeError(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, ListUsersResponse{Successful: true, Data: NewUserListData(users)})
}

// GetMe godoc
//
//	@ID			getMe
//	@Summary	Get the authenticated user
//	@Tags		user
//	@Produce	json
//	@Security	BearerAuth
//	@Success	200	{object}	UserResponse
//	@Failure	401	{object}	ErrorResponse	"UNAUTHORIZED"
//	@Failure	404	{object}	ErrorResponse	"USER_NOT_FOUND"
//	@Router		/api/v1/users/me [get]
func (h *UserHandler) GetMe(c *gin.Context) {
	h.respondUser(c, requestctx.UserID(c.Request.Context()))
}

// GetUserByID godoc
//
//	@ID			getUserByID
//	@Summary	Get a user by ID
//	@Tags		user
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id	path		string	true	"User ID (MongoDB ObjectID)"
//	@Success	200	{object}	UserResponse
//	@Failure	400	{object}	ErrorResponse	"INVALID_USER_ID"
//	@Failure	401	{object}	ErrorResponse	"UNAUTHORIZED"
//	@Failure	404	{object}	ErrorResponse	"USER_NOT_FOUND"
//	@Failure	500	{object}	ErrorResponse	"INTERNAL_SERVER_ERROR"
//	@Router		/api/v1/users/{id} [get]
func (h *UserHandler) GetUserByID(c *gin.Context) {
	h.respondUser(c, c.Param("id"))
}

func (h *UserHandler) respondUser(c *gin.Context, id string) {
	u, err := h.userService.GetUserByID(c.Request.Context(), id)
	if err != nil {
		writeError(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, UserResponse{Successful: true, Data: NewUserData(u)})
}

// PatchUserByID godoc
//
//	@ID				patchUserByID
//	@Summary		Update a user's name and/or email
//	@Description	Only keys present in the body are overwritten; at least one is required.
//	@Description	Users can only update their own account.
//	@Tags			user
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		string					true	"User ID (MongoDB ObjectID)"
//	@Param			body	body		PatchUserRequest	true	"Fields to update"
//	@Success		200		{object}	UserResponse
//	@Failure		400		{object}	ErrorResponse	"INVALID_REQUEST_BODY | INVALID_USER_ID | INVALID_NAME | INVALID_EMAIL"
//	@Failure		401		{object}	ErrorResponse	"UNAUTHORIZED"
//	@Failure		403		{object}	ErrorResponse	"FORBIDDEN"
//	@Failure		404		{object}	ErrorResponse	"USER_NOT_FOUND"
//	@Failure		409		{object}	ErrorResponse	"EMAIL_ALREADY_EXISTS"
//	@Failure		500		{object}	ErrorResponse	"INTERNAL_SERVER_ERROR"
//	@Router			/api/v1/users/{id} [patch]
func (h *UserHandler) PatchUserByID(c *gin.Context) {
	var req PatchUserRequest
	if err := decodeJSONBody(c, &req); err != nil {
		writeError(c, h.logger, err)
		return
	}
	actorID := requestctx.UserID(c.Request.Context())
	u, err := h.userService.PatchUserByID(c.Request.Context(), actorID, c.Param("id"), req.ToDomain())
	if err != nil {
		writeError(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, UserResponse{Successful: true, Data: NewUserData(u)})
}

// DeleteUserByID godoc
//
//	@ID				deleteUserByID
//	@Summary		Delete a user
//	@Description	Users can only delete their own account.
//	@Tags			user
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"User ID (MongoDB ObjectID)"
//	@Success		200	{object}	BaseResponse
//	@Failure		400	{object}	ErrorResponse	"INVALID_USER_ID"
//	@Failure		401	{object}	ErrorResponse	"UNAUTHORIZED"
//	@Failure		403	{object}	ErrorResponse	"FORBIDDEN"
//	@Failure		404	{object}	ErrorResponse	"USER_NOT_FOUND"
//	@Failure		500	{object}	ErrorResponse	"INTERNAL_SERVER_ERROR"
//	@Router			/api/v1/users/{id} [delete]
func (h *UserHandler) DeleteUserByID(c *gin.Context) {
	actorID := requestctx.UserID(c.Request.Context())
	if err := h.userService.DeleteUserByID(c.Request.Context(), actorID, c.Param("id")); err != nil {
		writeError(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, BaseResponse{Successful: true})
}
