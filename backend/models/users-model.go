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

// VerifyEmailRequest is the JSON body of POST /api/v1/user/verify-email.
type VerifyEmailRequest struct {
	Token string `json:"token"`
}

// ResendVerificationRequest is the JSON body of
// POST /api/v1/user/resend-verification.
type ResendVerificationRequest struct {
	Email string `json:"email"`
}

// ForgotPasswordRequest is the JSON body of
// POST /api/v1/user/forgot-password.
type ForgotPasswordRequest struct {
	Email string `json:"email"`
}

// ResetPasswordRequest is the JSON body of
// POST /api/v1/user/reset-password.
type ResetPasswordRequest struct {
	Token                string `json:"token"`
	Password             string `json:"password"`
	PasswordConfirmation string `json:"password_confirmation"`
}

// User is the public representation of a user, as returned by the API.
type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Role  string `json:"role"`

	// TokenVersion is the account's session generation. It is never part of an
	// API response; it is carried in the JWT and compared by the request guard.
	TokenVersion int `json:"-"`
}

// AuthInfo is returned by the session endpoints: the user plus when the
// session token expires (unix timestamp in seconds).
type AuthInfo struct {
	User
	ExpiresAt int64 `json:"expires_at"`
}

// Profile is the caller's own account, as returned by the profile endpoints.
type Profile struct {
	ID                int       `json:"id"`
	Name              string    `json:"name"`
	Email             string    `json:"email"`
	OrgID             string    `json:"org_id"`
	Role              string    `json:"role"`
	Confirmed         bool      `json:"confirmed"`
	CodeEditorEnabled bool      `json:"code_editor_enabled"`
	CreatedAt         time.Time `json:"created_at"`
	// PendingEmail is a requested email change awaiting verification. Empty when
	// no change is in flight.
	PendingEmail string `json:"pending_email"`
}

// UpdateProfileRequest is the JSON body of PUT /api/v1/user/profile. Every
// field is optional: absent fields are left untouched (partial update).
// CurrentPassword is required only when Email changes.
type UpdateProfileRequest struct {
	Name              *string `json:"name"`
	Email             *string `json:"email"`
	OrgID             *string `json:"org_id"`
	CodeEditorEnabled *bool   `json:"code_editor_enabled"`
	CurrentPassword   string  `json:"current_password"`
}

// ChangePasswordRequest is the JSON body of PUT /api/v1/user/password.
type ChangePasswordRequest struct {
	CurrentPassword         string `json:"current_password"`
	NewPassword             string `json:"new_password"`
	NewPasswordConfirmation string `json:"new_password_confirmation"`
}
