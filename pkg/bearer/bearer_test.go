package bearer_test

import (
	"testing"

	"github.com/get-phanuwat-api1/backend-challenge/pkg/bearer"
)

func TestParse(t *testing.T) {
	tests := []struct {
		header, token string
		ok            bool
	}{
		{"Bearer abc", "abc", true},
		{"bearer abc", "abc", true},
		{"  Bearer   abc  ", "abc", true},
		{"", "", false},
		{"Bearer", "", false},
		{"Bearer ", "", false},
		{"Basic abc", "", false},
		{"abc", "", false},
	}
	for _, tt := range tests {
		tok, ok := bearer.Parse(tt.header)
		if tok != tt.token || ok != tt.ok {
			t.Errorf("Parse(%q) = %q, %v; want %q, %v", tt.header, tok, ok, tt.token, tt.ok)
		}
	}
}
