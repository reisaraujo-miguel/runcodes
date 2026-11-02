package models

// RecompileResponse reports the outcome of a bulk recompilation.
type RecompileResponse struct {
	Queued int `json:"queued"`
	Failed int `json:"failed"`
}
