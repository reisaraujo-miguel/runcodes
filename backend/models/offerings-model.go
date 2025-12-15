package models

type Offering struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	EndDate        string `json:"end_date,omitempty"`
	EnrollmentCode string `json:"enrollment_code,omitempty"`
}

type CreateOfferingRequest struct {
	Email   string `json:"email"`
	Name    string `json:"name"`
	EndDate string `json:"end_date,omitempty"`
}
