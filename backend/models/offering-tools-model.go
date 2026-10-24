package models

// OfferingEmailRequest is the JSON body of POST
// /api/v1/offerings/{id}/notify.
type OfferingEmailRequest struct {
	Subject string `json:"subject"`
	Message string `json:"message"`
}

// OfferingEmailResponse reports how many members were queued for the email.
type OfferingEmailResponse struct {
	Recipients int `json:"recipients"`
}

// RecompileResponse reports the outcome of a bulk recompilation.
type RecompileResponse struct {
	Queued int `json:"queued"`
	Failed int `json:"failed"`
}

// OfferingEmailRequest
