package mocks

import (
	"errors"
	"strings"
	"time"

	"github.com/get-phanuwat-api1/backend-challenge/internal/application"
	"github.com/get-phanuwat-api1/backend-challenge/internal/domain"
)

// PasswordHasherFake "hashes" by adding a prefix, so tests stay fast and
// deterministic. Never use it outside tests.
type PasswordHasherFake struct {
	// Err, when set, is returned by Hash.
	Err error
}

var _ application.PasswordHasher = PasswordHasherFake{}

func (h PasswordHasherFake) Hash(password string) (string, error) {
	if h.Err != nil {
		return "", h.Err
	}
	return "hashed:" + password, nil
}

func (h PasswordHasherFake) Compare(hash, password string) error {
	if hash != "hashed:"+password {
		return errors.New("password mismatch")
	}
	return nil
}

// TokenManagerFake issues tokens of the form "token-for-<userID>".
type TokenManagerFake struct{}

var _ application.TokenManager = TokenManagerFake{}

func (TokenManagerFake) Generate(userID string) (domain.AccessToken, error) {
	return domain.AccessToken{Value: "token-for-" + userID, ExpiresAt: time.Now().Add(time.Hour)}, nil
}

func (TokenManagerFake) Validate(token string) (string, error) {
	userID, ok := strings.CutPrefix(token, "token-for-")
	if !ok || userID == "" {
		return "", errors.New("invalid token")
	}
	return userID, nil
}
