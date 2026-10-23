// Package bearer parses "Authorization: Bearer <token>" values.
package bearer

import "strings"

// Parse extracts the token from an "Authorization: Bearer <token>" header or
// gRPC metadata value. The scheme is matched case-insensitively.
func Parse(header string) (string, bool) {
	scheme, tok, ok := strings.Cut(strings.TrimSpace(header), " ")
	tok = strings.TrimSpace(tok)
	if !ok || !strings.EqualFold(scheme, "Bearer") || tok == "" {
		return "", false
	}
	return tok, true
}
