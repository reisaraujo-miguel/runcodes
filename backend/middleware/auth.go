// Package middleware provides HTTP middleware functions for request processing.
package middleware

import (
	"context"
	"net/http"
	"strings"

	"runcodes/errors"
	"runcodes/utils"
)

type contextKey string

const (
	UserIDKey    contextKey = "user_id"
	UserEmailKey contextKey = "user_email"
)

// AuthMiddleware validates JWT tokens and adds user context to requests
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenString := extractBearerToken(r)
		if tokenString == "" {
			errors.WriteError(w, errors.ErrUnauthorized)
			return
		}

		claims, err := utils.ValidateToken(tokenString)
		if err != nil {
			errors.LogError("auth.go", "AuthMiddleware", "Invalid or expired token", err)
			errors.WriteError(w, errors.NewAppError(errors.ErrUnauthorized.Code, "Invalid or expired token", err))
			return
		}

		if claims.UserID == "" || claims.Email == "" {
			errors.WriteError(w, errors.NewAppError(errors.ErrUnauthorized.Code, "Invalid token claims", nil))
			return
		}

		// Add user context to request
		ctx := r.Context()
		ctx = context.WithValue(ctx, UserIDKey, claims.UserID)
		ctx = context.WithValue(ctx, UserEmailKey, claims.Email)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// extractBearerToken extracts the Bearer token from the Authorization header
func extractBearerToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return ""
	}

	const prefix = "Bearer "
	if !strings.HasPrefix(strings.ToLower(authHeader), strings.ToLower(prefix)) {
		return ""
	}

	return strings.TrimSpace(authHeader[len(prefix):])
}

// GetUserIDFromContext retrieves the user ID from the request context
func GetUserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(UserIDKey).(string)
	return userID, ok
}

// GetUserEmailFromContext retrieves the user email from the request context
func GetUserEmailFromContext(ctx context.Context) (string, bool) {
	email, ok := ctx.Value(UserEmailKey).(string)
	return email, ok
}
