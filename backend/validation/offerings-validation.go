package validation

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"runcodes/models"
)

/*
CreateOfferingValidation validates the request fields
*/
func CreateOfferingValidation(req *models.CreateOfferingRequest, ctx context.Context) error {
	if err := ValidateRequiredString(req.Name, 100); err != nil {
		slog.ErrorContext(ctx, "invalid name", slog.String("Error", err.Error()))
		return err
	}

	if date, err := ValidateDate(ctx, req.EndDate); err != nil {
		slog.ErrorContext(ctx, "invalid date", slog.String("Error", err.Error()))
		return err
	} else if date.After(time.Now()) {
		msg := "end date can't be before the creation date"
		slog.ErrorContext(ctx, "invalid date", slog.String("Error", msg))
		return errors.New(msg)
	}

	return nil
}
