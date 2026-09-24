package services

import (
	"context"
	"log/slog"

	"github.com/runcodes-icmc/runcodes/cache"
	"github.com/runcodes-icmc/runcodes/database"
	"github.com/runcodes-icmc/runcodes/models"
	"github.com/runcodes-icmc/runcodes/validation"
)

// MaxContactDisclaimerLength bounds the admin-supplied disclaimer. The text is
// rendered as HTML on the public login page, so it is sanitized by the client
// before insertion; the cap is what keeps a runaway value out of the database.
// An empty disclaimer is valid and hides it from the login page.
const MaxContactDisclaimerLength = 4000

// Defaults served when a key is missing from platform_settings (a database
// seeded before the settings existed, or a deleted row).
const (
	defaultContactEmail          = "contact@example.com"
	defaultContactDisclaimerHTML = `Em caso de eventuais problemas com a plataforma, entre em contato com <a href="mailto:contact@example.com">contact@example.com</a>`
)

/*
GetSettings returns the platform settings. It is public: the login page reads
the contact information before anyone is authenticated.
*/
func GetSettings(ctx context.Context) (*models.PlatformSettings, error) {
	settings := models.PlatformSettings{
		ContactEmail:          defaultContactEmail,
		ContactDisclaimerHTML: defaultContactDisclaimerHTML,
	}

	if cache.GetJSON(ctx, cache.KeyPlatformSettings, &settings) {
		return &settings, nil
	}

	rows, err := database.DB.QueryContext(ctx,
		"SELECT key, value FROM platform_settings")
	if err != nil {
		slog.ErrorContext(ctx, "error fetching platform settings",
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	defer rows.Close()

	found := false
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			slog.ErrorContext(ctx, "error scanning platform setting",
				slog.String("error", err.Error()),
			)
			return nil, ErrServer
		}
		found = true

		switch key {
		case models.SettingContactEmail:
			// An empty address would leave users with no way to reach the
			// platform, so the default is kept instead.
			if value != "" {
				settings.ContactEmail = value
			}
		case models.SettingContactDisclaimerHTML:
			// Stored as submitted: the admin may have cleared it on purpose,
			// and the login page renders nothing in that case.
			settings.ContactDisclaimerHTML = value
		}
	}
	if err := rows.Err(); err != nil {
		slog.ErrorContext(ctx, "error iterating platform settings",
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}

	// A row-less settings table means a database that predates them; serving
	// the defaults (and not caching them) keeps the page usable.
	if found {
		cache.SetJSON(ctx, cache.KeyPlatformSettings, settings, cache.SettingsTTL)
	}

	return &settings, nil
}

/*
ValidateSettings validates the settings an admin submits. The contact address is
required, because an empty one would leave users with no way to reach the
platform; the disclaimer may be empty, which hides it on the login page.
*/
func ValidateSettings(ctx context.Context, req *models.PlatformSettings) error {
	if err := validation.ValidateOptionalString(
		req.ContactDisclaimerHTML, MaxContactDisclaimerLength,
	); err != nil {
		return err
	}
	return validation.ValidateEmailFormat(ctx, req.ContactEmail)
}

/*
UpdateSettings stores the platform settings and invalidates the public cache
entry, so the change is visible on the next request.
*/
func UpdateSettings(
	ctx context.Context, req *models.PlatformSettings,
) (*models.PlatformSettings, error) {
	if err := ValidateSettings(ctx, req); err != nil {
		return nil, err
	}

	tx, err := database.DB.BeginTx(ctx, nil)
	if err != nil {
		slog.ErrorContext(ctx, "error initializing settings transaction",
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	defer tx.Rollback()

	pairs := []struct {
		key   string
		value string
	}{
		{models.SettingContactEmail, req.ContactEmail},
		{models.SettingContactDisclaimerHTML, req.ContactDisclaimerHTML},
	}

	for _, pair := range pairs {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO platform_settings (key, value, updated_at)
			VALUES ($1, $2, now())
			ON CONFLICT (key) DO UPDATE
			SET value = EXCLUDED.value, updated_at = now()`, pair.key, pair.value,
		); err != nil {
			slog.ErrorContext(ctx, "error upserting platform setting",
				slog.String("key", pair.key),
				slog.String("error", err.Error()),
			)
			return nil, ErrServer
		}
	}

	if err := tx.Commit(); err != nil {
		slog.ErrorContext(ctx, "error committing settings update",
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}

	cache.Delete(ctx, cache.KeyPlatformSettings)

	settings, err := GetSettings(ctx)
	if err != nil {
		// The write succeeded even if the read-back failed; report what was
		// stored rather than failing the request.
		slog.WarnContext(ctx, "error reading settings back after update",
			slog.String("error", err.Error()),
		)
		return req, nil
	}
	return settings, nil
}
