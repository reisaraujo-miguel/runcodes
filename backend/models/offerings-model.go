package models

type CreateOfferingRequest struct {
	Name        string `json:"name"`
	EndDate     string `json:"end_date"`
	Description string `json:"description"`
}

// Offering is a class offering as returned by the API.
type Offering struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	EndDate        string `json:"end_date"`
	Description    string `json:"description"`
	EnrollmentCode string `json:"enrollment_code"`
}
