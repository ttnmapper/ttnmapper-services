package utils

import (
	"strings"
	"testing"
)

// These malformed addresses must be rejected before any DNS lookup.
func TestValidateEmailRejectsMalformedAddresses(t *testing.T) {
	for _, tc := range []struct{ name, email, message string }{
		{"empty", "", "email address is empty"},
		{"missing at", "mapper", "email doesn't contain an @"},
		{"missing user", "@example.invalid", "email no user part"},
		{"long user", strings.Repeat("a", 65) + "@example.invalid", "email user part exceeds max length"},
		{"missing domain", "mapper@", "email no domain part"},
		{"long domain", "mapper@" + strings.Repeat("a", 256), "email domain part exceeds max length"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateEmail(tc.email)
			if err == nil || err.Error() != tc.message {
				t.Fatalf("ValidateEmail() = %v, want %q", err, tc.message)
			}
		})
	}
}
