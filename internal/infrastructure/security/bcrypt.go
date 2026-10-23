package security

import (
	"golang.org/x/crypto/bcrypt"

	"github.com/get-phanuwat-api1/backend-challenge/internal/application"
)

// BcryptHasher hashes passwords with bcrypt.
type BcryptHasher struct {
	cost int
}

var _ application.PasswordHasher = BcryptHasher{}

// NewBcryptHasher returns a hasher using the given cost (bcrypt.DefaultCost
// in production, bcrypt.MinCost in tests).
func NewBcryptHasher(cost int) BcryptHasher {
	return BcryptHasher{cost: cost}
}

// Hash returns the bcrypt hash of password.
func (h BcryptHasher) Hash(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), h.cost)
	return string(b), err
}

// Compare returns a non-nil error if password does not match hash.
func (h BcryptHasher) Compare(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}
