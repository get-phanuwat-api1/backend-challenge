package domain_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/get-phanuwat-api1/backend-challenge/internal/domain"
)

func TestError_IsMatchesByCode(t *testing.T) {
	err := fmt.Errorf("wrapped: %w", domain.NewError(domain.CodeUserNotFound, "custom message"))
	if !errors.Is(err, domain.ErrUserNotFound) {
		t.Error("errors with the same code should match")
	}
	if errors.Is(err, domain.ErrForbidden) {
		t.Error("errors with different codes should not match")
	}
	if errors.Is(errors.New("plain"), domain.ErrForbidden) {
		t.Error("plain errors should not match")
	}
}

func TestCodeOf(t *testing.T) {
	if got := domain.CodeOf(fmt.Errorf("x: %w", domain.ErrForbidden)); got != domain.CodeForbidden {
		t.Errorf("got %s", got)
	}
	if got := domain.CodeOf(errors.New("boom")); got != domain.CodeInternal {
		t.Errorf("unknown errors should be internal, got %s", got)
	}
}

func TestError_Message(t *testing.T) {
	if got := domain.ErrUserNotFound.Error(); got != "USER_NOT_FOUND: user not found" {
		t.Errorf("Error() = %q", got)
	}
}
