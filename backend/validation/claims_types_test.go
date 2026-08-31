package validation

import (
	"testing"
	"time"

	"github.com/go-chi/jwtauth/v5"
	"github.com/lestrrat-go/jwx/v3/transform"
)

// TestDecodedClaimTypes pins the types that jwtauth/jwx decode claims into,
// so handlers that assert on them (e.g. GetAuth reading "exp") don't break
// silently when the libraries change.
func TestDecodedClaimTypes(t *testing.T) {
	TokenAuth = jwtauth.New("HS256", []byte("test-secret"), nil)

	claims := map[string]any{
		"id":    42,
		"name":  "Test",
		"email": "t@t.com",
		"role":  "student",
	}
	jwtauth.SetIssuedAt(claims, time.Now())
	jwtauth.SetExpiryIn(claims, 30*time.Minute)

	_, tokenStr, err := TokenAuth.Encode(claims)
	if err != nil {
		t.Fatal(err)
	}

	token, err := TokenAuth.Decode(tokenStr)
	if err != nil {
		t.Fatal(err)
	}

	decoded := map[string]any{}
	if err := transform.AsMap(token, decoded); err != nil {
		t.Fatal(err)
	}

	if _, ok := decoded["id"].(float64); !ok {
		t.Errorf("expected id claim to decode as float64, got %T", decoded["id"])
	}
	for _, key := range []string{"name", "email", "role"} {
		if _, ok := decoded[key].(string); !ok {
			t.Errorf("expected %s claim to decode as string, got %T", key, decoded[key])
		}
	}
	if _, ok := decoded["exp"].(time.Time); !ok {
		t.Errorf("expected exp claim to decode as time.Time, got %T", decoded["exp"])
	}
}
