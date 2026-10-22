package validation

import (
	"errors"
	"strings"
	"testing"
)

func TestValidatePasswordLengthBounds(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  error
	}{
		{"minimum length", "Aa1!aaaa", nil},
		{"too short", "Aa1!aaa", ErrInputTooShort},
		{"exactly 72 bytes", strings.Repeat("Aa1!", 18), nil},
		{"over 72 bytes", strings.Repeat("Aa1!", 18) + "a", ErrPasswordTooLong},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidatePassword(tt.password); !errors.Is(err, tt.wantErr) {
				t.Fatalf("ValidatePassword(len=%d) = %v, want %v",
					len(tt.password), err, tt.wantErr)
			}
		})
	}
}
