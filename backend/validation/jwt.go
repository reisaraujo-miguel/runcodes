package validation

import (
	"errors"
	"log/slog"
	"time"

	"github.com/runcodes-icmc/runcodes/config"

	"github.com/go-chi/jwtauth/v5"
	"github.com/lestrrat-go/jwx/v3/jwt"
)

var TokenAuth *jwtauth.JWTAuth

// SessionTTL is how long an auth session token stays valid. Sessions are
// renewed (sliding) by POST /api/v1/auth/refresh while the user is active.
const SessionTTL = 30 * time.Minute

/*
SetupJWT creates a new jwtauth from the configured secret, exposing it as
TokenAuth.
*/
func SetupJWT() error {
	secret := config.Get().JWTSecret

	if secret == "" {
		err := errors.New("RUNCODES_JWT_SECRET is not set")
		slog.Error(err.Error())
		return err
	}

	TokenAuth = jwtauth.New("HS256",
		[]byte(secret), nil, jwt.WithAcceptableSkew(30*time.Second),
	)

	return nil
}
