// Request and response shapes (Data Transfer Objects). Every response uses
// the same envelope: {"successful", "error_code", "data"}.

package httpapi

import (
	"time"

	"github.com/get-phanuwat-api1/backend-challenge/internal/domain"
)

// BaseResponse is returned by endpoints that have no data (e.g. DELETE).
type BaseResponse struct {
	Successful bool   `json:"successful" example:"true"`
	ErrorCode  string `json:"error_code" example:""`
}

// ErrorResponse is returned by every endpoint on failure. Data is always null.
type ErrorResponse struct {
	Successful bool   `json:"successful" example:"false"`
	ErrorCode  string `json:"error_code" example:"INVALID_EMAIL"`
	Data       any    `json:"data" swaggertype:"object" extensions:"x-nullable"`
}

// NewErrorResponse builds an ErrorResponse for code.
func NewErrorResponse(code string) ErrorResponse {
	return ErrorResponse{Successful: false, ErrorCode: code, Data: nil}
}

// CreateUserRequest is the body of POST /api/v1/users and POST /api/v1/auth/register.
type CreateUserRequest struct {
	Name     string `json:"name" example:"Alice"`
	Email    string `json:"email" example:"alice@example.com"`
	Password string `json:"password" example:"password123"`
}

// PatchUserRequest is the body of PATCH /api/v1/users/{id}.
// Omitted keys are left untouched. At least one key is required.
type PatchUserRequest struct {
	Name  *string `json:"name" example:"Alicia" extensions:"x-nullable=false"`
	Email *string `json:"email" example:"alicia@example.com" extensions:"x-nullable=false"`
}

// ToDomain converts the request into a domain.UserPatch.
func (r PatchUserRequest) ToDomain() domain.UserPatch {
	return domain.UserPatch{Name: r.Name, Email: r.Email}
}

// UserData is the user representation returned in responses. The password
// hash is never exposed.
type UserData struct {
	ID        string    `json:"id" example:"6ad83e6db6f99c0a372a5356"`
	Name      string    `json:"name" example:"Alice"`
	Email     string    `json:"email" example:"alice@example.com"`
	CreatedAt time.Time `json:"created_at" example:"2026-10-21T04:24:13.532Z"`
}

// NewUserData maps a domain.User to its response representation.
func NewUserData(u *domain.User) *UserData {
	return &UserData{ID: u.ID, Name: u.Name, Email: u.Email, CreatedAt: u.CreatedAt}
}

// UserResponse is returned by endpoints that return a single user.
type UserResponse struct {
	Successful bool      `json:"successful" example:"true"`
	ErrorCode  string    `json:"error_code" example:""`
	Data       *UserData `json:"data" extensions:"x-nullable"`
}

// UserListData is the payload of GET /api/v1/users.
type UserListData struct {
	Users []UserData `json:"users"`
	Total int        `json:"total" example:"2"`
}

// NewUserListData maps users to their response representation.
func NewUserListData(users []domain.User) *UserListData {
	data := &UserListData{Users: make([]UserData, len(users)), Total: len(users)}
	for i := range users {
		data.Users[i] = *NewUserData(&users[i])
	}
	return data
}

// ListUsersResponse is the response of GET /api/v1/users.
type ListUsersResponse struct {
	Successful bool          `json:"successful" example:"true"`
	ErrorCode  string        `json:"error_code" example:""`
	Data       *UserListData `json:"data" extensions:"x-nullable"`
}

// LoginRequest is the body of POST /api/v1/auth/login.
type LoginRequest struct {
	Email    string `json:"email" example:"alice@example.com"`
	Password string `json:"password" example:"password123"`
}

// TokenData is the payload returned after a successful login.
type TokenData struct {
	AccessToken string    `json:"access_token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
	TokenType   string    `json:"token_type" example:"Bearer"`
	ExpiresAt   time.Time `json:"expires_at" example:"2026-10-22T04:24:13Z"`
}

// NewTokenData maps an issued token to its response representation.
func NewTokenData(t domain.AccessToken) *TokenData {
	return &TokenData{AccessToken: t.Value, TokenType: "Bearer", ExpiresAt: t.ExpiresAt}
}

// LoginResponse is the response of POST /api/v1/auth/login.
type LoginResponse struct {
	Successful bool       `json:"successful" example:"true"`
	ErrorCode  string     `json:"error_code" example:""`
	Data       *TokenData `json:"data" extensions:"x-nullable"`
}
