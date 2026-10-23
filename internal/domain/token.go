package domain

import "time"

// AccessToken is an issued authentication token.
type AccessToken struct {
	Value     string
	ExpiresAt time.Time
}
