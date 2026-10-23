// Package security holds driven adapters for the security ports:
// JWT (HS256) for out.TokenManager and bcrypt for out.PasswordHasher.
package security

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/get-phanuwat-api1/backend-challenge/internal/application"
	"github.com/get-phanuwat-api1/backend-challenge/internal/domain"
)

// MinSecretLength is the minimum HMAC key size (256 bits for HS256).
const MinSecretLength = 32

// ErrInvalidToken is returned for any token that fails validation.
var ErrInvalidToken = errors.New("invalid or expired token")

// JWTManager issues and validates HS256-signed JWTs.
type JWTManager struct {
	secret []byte
	issuer string
	ttl    time.Duration
	now    func() time.Time
}

var _ application.TokenManager = (*JWTManager)(nil)

// NewJWTManager returns a JWTManager. The secret must be at least 32 bytes.
func NewJWTManager(secret []byte, issuer string, ttl time.Duration) (*JWTManager, error) {
	if len(secret) < MinSecretLength {
		return nil, errors.New("jwt secret must be at least 32 bytes")
	}
	if ttl <= 0 {
		return nil, errors.New("jwt ttl must be positive")
	}
	return &JWTManager{secret: secret, issuer: issuer, ttl: ttl, now: time.Now}, nil
}

// Generate issues a token whose subject is userID.
func (m *JWTManager) Generate(userID string) (domain.AccessToken, error) {
	now := m.now()
	expiresAt := now.Add(m.ttl)
	claims := jwt.RegisteredClaims{
		Subject:   userID,
		Issuer:    m.issuer,
		IssuedAt:  jwt.NewNumericDate(now),
		NotBefore: jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(expiresAt),
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
	if err != nil {
		return domain.AccessToken{}, err
	}
	return domain.AccessToken{Value: signed, ExpiresAt: expiresAt.Truncate(time.Second)}, nil
}

// Validate verifies the signature, algorithm, issuer and expiry, and returns
// the token subject (user ID).
func (m *JWTManager) Validate(token string) (string, error) {
	claims := &jwt.RegisteredClaims{}
	_, err := jwt.ParseWithClaims(token, claims,
		func(*jwt.Token) (any, error) { return m.secret, nil },
		// Pinning the algorithm prevents "alg: none" and algorithm-confusion attacks.
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(m.issuer),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(m.now),
	)
	if err != nil || claims.Subject == "" {
		return "", ErrInvalidToken
	}
	return claims.Subject, nil
}
