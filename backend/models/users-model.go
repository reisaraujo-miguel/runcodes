package models

type SignInRequest struct {
	UserName             string `json:"name"`
	Email                string `json:"email"`
	Password             string `json:"password"`
	PasswordConfirmation string `json:"password-confirmation"`
}

type LogInRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}
