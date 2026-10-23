// Package domain is the Domain layer: entities, their business rules and
// business errors. It depends on nothing but the standard library.
package domain

import (
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	MaxNameLength     = 100
	MaxEmailLength    = 254
	MinPasswordLength = 8
	// MaxPasswordLength is bcrypt's input limit in bytes.
	MaxPasswordLength = 72
)

// User is the core entity.
type User struct {
	ID           string
	Name         string
	Email        string
	PasswordHash string
	CreatedAt    time.Time
}

// NewUser builds a validated User from raw input. Name is trimmed and email
// is normalized. The password is validated but not hashed: PasswordHash is
// left for the caller to set.
func NewUser(name, email, password string) (*User, error) {
	u := &User{
		Name:  strings.TrimSpace(name),
		Email: NormalizeEmail(email),
	}
	if err := u.Validate(); err != nil {
		return nil, err
	}
	if err := ValidatePassword(password); err != nil {
		return nil, err
	}
	return u, nil
}

// Validate enforces user business rules on name and email.
func (u *User) Validate() error {
	if err := validateName(u.Name); err != nil {
		return err
	}
	return validateEmail(u.Email)
}

// UserPatch describes a partial update. Nil fields are left untouched.
type UserPatch struct {
	Name  *string
	Email *string
}

// IsEmpty reports whether no field was provided.
func (p UserPatch) IsEmpty() bool { return p.Name == nil && p.Email == nil }

// ApplyPatch overwrites only the fields present in the patch, then validates the result.
// The receiver is left unchanged when an error is returned.
func (u *User) ApplyPatch(patch UserPatch) error {
	if patch.IsEmpty() {
		return NewError(CodeInvalidRequestBody, "at least one of name or email is required")
	}
	next := *u
	if patch.Name != nil {
		next.Name = strings.TrimSpace(*patch.Name)
	}
	if patch.Email != nil {
		next.Email = NormalizeEmail(*patch.Email)
	}
	if err := next.Validate(); err != nil {
		return err
	}
	*u = next
	return nil
}

// NormalizeEmail trims and lowercases an email so uniqueness is case-insensitive.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// ValidatePassword enforces password length rules.
func ValidatePassword(password string) error {
	switch {
	case password == "":
		return NewError(CodeInvalidPassword, "password is required")
	case utf8.RuneCountInString(password) < MinPasswordLength:
		return NewError(CodeInvalidPassword, "password must be at least 8 characters")
	case len(password) > MaxPasswordLength:
		return NewError(CodeInvalidPassword, "password must be at most 72 bytes")
	}
	return nil
}

func validateName(name string) error {
	switch {
	case name == "":
		return NewError(CodeInvalidName, "name is required")
	case utf8.RuneCountInString(name) > MaxNameLength:
		return NewError(CodeInvalidName, "name must not exceed 100 characters")
	}
	return nil
}

func validateEmail(email string) error {
	if email == "" {
		return NewError(CodeInvalidEmail, "email is required")
	}
	if len(email) > MaxEmailLength {
		return NewError(CodeInvalidEmail, "email is too long")
	}
	// ParseAddress also accepts `Name <a@b.c>`; require the bare address with a dotted domain.
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email || !strings.Contains(email[strings.LastIndex(email, "@")+1:], ".") {
		return NewError(CodeInvalidEmail, "email must be a valid email address")
	}
	return nil
}
