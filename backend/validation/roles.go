package validation

import (
	"net/http"

	"github.com/go-chi/jwtauth/v5"
)

/*
RequireRole restricts a route to the given roles. It must be used after
jwtauth.Verifier and jwtauth.Authenticator so the claims are in the context.
*/
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, claims, err := jwtauth.FromContext(r.Context())
			if err != nil {
				http.Error(w,
					http.StatusText(http.StatusUnauthorized),
					http.StatusUnauthorized,
				)
				return
			}

			role, ok := claims["role"].(string)
			if !ok {
				http.Error(w,
					http.StatusText(http.StatusForbidden),
					http.StatusForbidden,
				)
				return
			}

			for _, allowed := range roles {
				if role == allowed {
					next.ServeHTTP(w, r)
					return
				}
			}

			http.Error(w,
				http.StatusText(http.StatusForbidden),
				http.StatusForbidden,
			)
		})
	}
}
