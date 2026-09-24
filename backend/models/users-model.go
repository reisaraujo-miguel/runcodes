package models

import "time"

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

// Profile is the caller's own account, as returned by the profile endpoints.
type Profile struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	OrgID     string    `json:"org_id"`
	Role      string    `json:"role"`
	Confirmed bool      `json:"confirmed"`
	CreatedAt time.Time `json:"created_at"`
}

// UpdateProfileRequest is the JSON body of PUT /api/v1/user/profile. Every
// field is optional: absent fields are left untouched (partial update).
type UpdateProfileRequest struct {
	Name  *string `json:"name"`
	Email *string `json:"email"`
	OrgID *string `json:"org_id"`
}

// ChangePasswordRequest is the JSON body of PUT /api/v1/user/password.
type ChangePasswordRequest struct {
	CurrentPassword         string `json:"current_password"`
	NewPassword             string `json:"new_password"`
	NewPasswordConfirmation string `json:"new_password_confirmation"`
}
