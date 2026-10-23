package httpapi

import (
	"net/http"
	"testing"

	"github.com/get-phanuwat-api1/backend-challenge/internal/domain"
)

func TestHTTPStatus(t *testing.T) {
	tests := map[domain.ErrorCode]int{
		domain.CodeInvalidRequestBody: http.StatusBadRequest,
		domain.CodeInvalidUserID:      http.StatusBadRequest,
		domain.CodeInvalidEmail:       http.StatusBadRequest,
		domain.CodeInvalidCredentials: http.StatusUnauthorized,
		domain.CodeUnauthorized:       http.StatusUnauthorized,
		domain.CodeForbidden:          http.StatusForbidden,
		domain.CodeUserNotFound:       http.StatusNotFound,
		domain.CodeEmailAlreadyExists: http.StatusConflict,
		domain.CodeInternal:           http.StatusInternalServerError,
	}
	for code, want := range tests {
		if got := httpStatus(code); got != want {
			t.Errorf("httpStatus(%s) = %d, want %d", code, got, want)
		}
	}
}
