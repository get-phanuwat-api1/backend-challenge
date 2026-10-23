package security_test

import (
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/get-phanuwat-api1/backend-challenge/internal/infrastructure/security"
)

func TestBcryptHasher(t *testing.T) {
	h := security.NewBcryptHasher(bcrypt.MinCost)
	hash, err := h.Hash("password123")
	if err != nil {
		t.Fatal(err)
	}
	if hash == "password123" {
		t.Fatal("hash must not equal the plaintext")
	}
	if err := h.Compare(hash, "password123"); err != nil {
		t.Errorf("Compare with correct password: %v", err)
	}
	if err := h.Compare(hash, "wrong"); err == nil {
		t.Error("Compare with wrong password should fail")
	}
}
