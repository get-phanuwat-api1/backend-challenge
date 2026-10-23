package security

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var testSecret = []byte("0123456789abcdef0123456789abcdef")

func newManager(t *testing.T) *JWTManager {
	t.Helper()
	m, err := NewJWTManager(testSecret, "test-issuer", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestNewJWTManager_RejectsWeakConfig(t *testing.T) {
	if _, err := NewJWTManager([]byte("short"), "x", time.Hour); err == nil {
		t.Error("expected error for short secret")
	}
	if _, err := NewJWTManager(testSecret, "x", 0); err == nil {
		t.Error("expected error for zero ttl")
	}
}

func TestJWTManager_RoundTrip(t *testing.T) {
	m := newManager(t)
	tok, err := m.Generate("user-1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(tok.Value, "eyJhbGciOiJIUzI1NiIs") { // {"alg":"HS256",...
		t.Errorf("token is not HS256: %s", tok.Value)
	}
	if d := time.Until(tok.ExpiresAt); d < 59*time.Minute || d > time.Hour {
		t.Errorf("unexpected expiry %v", tok.ExpiresAt)
	}
	sub, err := m.Validate(tok.Value)
	if err != nil || sub != "user-1" {
		t.Fatalf("Validate = %q, %v", sub, err)
	}
}

func TestJWTManager_RejectsInvalidTokens(t *testing.T) {
	m := newManager(t)
	now := time.Now()

	sign := func(method jwt.SigningMethod, key any, claims jwt.RegisteredClaims) string {
		s, err := jwt.NewWithClaims(method, claims).SignedString(key)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	valid := jwt.RegisteredClaims{
		Subject:   "user-1",
		Issuer:    "test-issuer",
		ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
	}
	expired := valid
	expired.ExpiresAt = jwt.NewNumericDate(now.Add(-time.Minute))
	noExp := valid
	noExp.ExpiresAt = nil
	wrongIssuer := valid
	wrongIssuer.Issuer = "someone-else"
	noSubject := valid
	noSubject.Subject = ""

	good, _ := m.Generate("user-1")
	tampered := good.Value[:len(good.Value)-2] + "xx"

	tests := map[string]string{
		"garbage":       "not-a-jwt",
		"wrong secret":  sign(jwt.SigningMethodHS256, []byte("another-secret-another-secret-!!"), valid),
		"alg none":      sign(jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType, valid),
		"HS512":         sign(jwt.SigningMethodHS512, testSecret, valid),
		"expired":       sign(jwt.SigningMethodHS256, testSecret, expired),
		"no expiry":     sign(jwt.SigningMethodHS256, testSecret, noExp),
		"wrong issuer":  sign(jwt.SigningMethodHS256, testSecret, wrongIssuer),
		"no subject":    sign(jwt.SigningMethodHS256, testSecret, noSubject),
		"bad signature": tampered,
	}
	for name, tok := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := m.Validate(tok); !errors.Is(err, ErrInvalidToken) {
				t.Fatalf("got %v, want ErrInvalidToken", err)
			}
		})
	}
}
