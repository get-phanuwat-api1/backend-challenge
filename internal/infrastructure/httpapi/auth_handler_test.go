package httpapi_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/get-phanuwat-api1/backend-challenge/internal/application"
	"github.com/get-phanuwat-api1/backend-challenge/internal/domain"
	"github.com/get-phanuwat-api1/backend-challenge/internal/mocks"
)

func TestRegister_Success(t *testing.T) {
	var got application.CreateUserInput
	authSvc := &mocks.AuthService{
		RegisterFunc: func(_ context.Context, input application.CreateUserInput) (*domain.User, error) {
			got = input
			return testUser, nil
		},
	}
	w := do(newEngine(authSvc, nil), http.MethodPost, "/auth/register",
		`{"name":"Alice","email":"alice@example.com","password":"password123"}`)

	expect(t, w, http.StatusCreated, `{"successful":true,"error_code":"","data":`+userJSON+`}`)
	if got != (application.CreateUserInput{Name: "Alice", Email: "alice@example.com", Password: "password123"}) {
		t.Errorf("service input = %+v", got)
	}
	if loc := w.Header().Get("Location"); loc != "/api/v1/users/"+testUserID {
		t.Errorf("Location = %q", loc)
	}
}

func TestRegister_Errors(t *testing.T) {
	tests := []struct {
		name   string
		body   string
		svcErr error
		status int
		code   string
	}{
		{"empty body", ``, nil, http.StatusBadRequest, "INVALID_REQUEST_BODY"},
		{"invalid json", `{`, nil, http.StatusBadRequest, "INVALID_REQUEST_BODY"},
		{"unknown field", `{"name":"A","admin":true}`, nil, http.StatusBadRequest, "INVALID_REQUEST_BODY"},
		{"two objects", `{}{}`, nil, http.StatusBadRequest, "INVALID_REQUEST_BODY"},
		{"validation", `{}`, domain.NewError(domain.CodeInvalidEmail, "x"), http.StatusBadRequest, "INVALID_EMAIL"},
		{"duplicate", `{}`, domain.ErrEmailAlreadyExists, http.StatusConflict, "EMAIL_ALREADY_EXISTS"},
		{"internal", `{}`, errBoom, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authSvc := &mocks.AuthService{
				RegisterFunc: func(context.Context, application.CreateUserInput) (*domain.User, error) { return nil, tt.svcErr },
			}
			w := do(newEngine(authSvc, nil), http.MethodPost, "/auth/register", tt.body)
			expect(t, w, tt.status, errorJSON(tt.code))
		})
	}
}

func TestLogin(t *testing.T) {
	expires := time.Date(2026, 10, 22, 4, 24, 13, 0, time.UTC)
	authSvc := &mocks.AuthService{
		LoginFunc: func(_ context.Context, email, password string) (domain.AccessToken, error) {
			if email == "alice@example.com" && password == "password123" {
				return domain.AccessToken{Value: "jwt-value", ExpiresAt: expires}, nil
			}
			return domain.AccessToken{}, domain.ErrInvalidCredentials
		},
	}
	r := newEngine(authSvc, nil)

	w := do(r, http.MethodPost, "/auth/login", `{"email":"alice@example.com","password":"password123"}`)
	expect(t, w, http.StatusOK,
		`{"successful":true,"error_code":"","data":{"access_token":"jwt-value","token_type":"Bearer","expires_at":"2026-10-22T04:24:13Z"}}`)

	w = do(r, http.MethodPost, "/auth/login", `{"email":"alice@example.com","password":"wrong"}`)
	expect(t, w, http.StatusUnauthorized, errorJSON("INVALID_CREDENTIALS"))

	w = do(r, http.MethodPost, "/auth/login", `not json`)
	expect(t, w, http.StatusBadRequest, errorJSON("INVALID_REQUEST_BODY"))
}
