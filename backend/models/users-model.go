package models

type SignUpRequest struct {
	Name                 string `json:"name"`
	Email                string `json:"email"`
	Password             string `json:"password"`
	PasswordConfirmation string `json:"password_confirmation"`
}

type LogInRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// User is the public representation of a user, as returned by the API.
type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

// AuthInfo is returned by the session endpoints: the user plus when the
// session token expires (unix timestamp in seconds).
type AuthInfo struct {
	User
	ExpiresAt int64 `json:"expires_at"`
}
