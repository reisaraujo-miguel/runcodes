package models

type CreateOfferingRequest struct {
	Name        string `json:"name"`
	EndDate     string `json:"end_date"`
	Description string `json:"description"`
}
