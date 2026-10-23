package domain

import "errors"

// ErrorCode is a machine readable business error code. Driving adapters map
// it to their own status (HTTP status, gRPC code); the domain knows neither.
type ErrorCode string

const (
	CodeInvalidRequestBody ErrorCode = "INVALID_REQUEST_BODY"
	CodeInvalidUserID      ErrorCode = "INVALID_USER_ID"
	CodeInvalidName        ErrorCode = "INVALID_NAME"
	CodeInvalidEmail       ErrorCode = "INVALID_EMAIL"
	CodeInvalidPassword    ErrorCode = "INVALID_PASSWORD"
	CodeInvalidCredentials ErrorCode = "INVALID_CREDENTIALS"
	CodeUnauthorized       ErrorCode = "UNAUTHORIZED"
	CodeForbidden          ErrorCode = "FORBIDDEN"
	CodeUserNotFound       ErrorCode = "USER_NOT_FOUND"
	CodeEmailAlreadyExists ErrorCode = "EMAIL_ALREADY_EXISTS"
	CodeInternal           ErrorCode = "INTERNAL_SERVER_ERROR"
)

// Error is a business error carrying an ErrorCode.
type Error struct {
	Code    ErrorCode
	Message string
}

func (e *Error) Error() string { return string(e.Code) + ": " + e.Message }

// Is lets errors.Is match errors by code.
func (e *Error) Is(target error) bool {
	var t *Error
	if !errors.As(target, &t) {
		return false
	}
	return t.Code == e.Code
}

// NewError creates a new business error.
func NewError(code ErrorCode, msg string) *Error { return &Error{Code: code, Message: msg} }

// Common business errors.
var (
	ErrUserNotFound       = NewError(CodeUserNotFound, "user not found")
	ErrEmailAlreadyExists = NewError(CodeEmailAlreadyExists, "email already exists")
	ErrInvalidUserID      = NewError(CodeInvalidUserID, "invalid user id")
	ErrInvalidCredentials = NewError(CodeInvalidCredentials, "invalid email or password")
	ErrUnauthorized       = NewError(CodeUnauthorized, "missing or invalid access token")
	ErrForbidden          = NewError(CodeForbidden, "you are not allowed to modify this user")
)

// CodeOf extracts the ErrorCode of err, falling back to INTERNAL_SERVER_ERROR.
func CodeOf(err error) ErrorCode {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return CodeInternal
}
