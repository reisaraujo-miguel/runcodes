package validation

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/go-chi/jwtauth/v5"
	"github.com/lestrrat-go/jwx/v3/jwt"
)

var TokenAuth *jwtauth.JWTAuth

/*
SetupJWT reads the JWT secret from the environment and creates a new jwtauth
that can be accessed via the validation.TokenAuth variable
*/
func SetupJWT() error {
	secret := []byte(os.Getenv("RUNCODES_JWT_SECRET"))

	if secret == nil {
		err := fmt.Errorf("RUNCODES_JWT_SECRET is not set")
		slog.Error(err.Error())
		return err
	}

	TokenAuth = jwtauth.New("HS256", secret, nil, jwt.WithAcceptableSkew(30*time.Second))

	return nil
}
