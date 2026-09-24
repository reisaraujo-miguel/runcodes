package models

// PlatformSettings holds the platform-wide values the admin edits from the
// admin panel. The keys are the same strings stored in platform_settings.
type PlatformSettings struct {
	ContactEmail          string `json:"contact_email"`
	ContactDisclaimerHTML string `json:"contact_disclaimer_html"`
}

// Setting keys stored in platform_settings.
const (
	SettingContactEmail          = "contact_email"
	SettingContactDisclaimerHTML = "contact_disclaimer_html"
)
