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
