// Package services provides business logic services for the application.
package services

import (
	"context"
	"database/sql"
	"log/slog"

	"runcodes/models"
)

/*
CreateOffering creates a new offering on the platform.
*/
func CreateOffering(
	ctx context.Context, req *models.CreateOfferingRequest, claims map[string]any,
) error {
	ownerIDRaw, ok := claims["id"]
	if !ok {
		slog.ErrorContext(ctx, "missing user id claim")
		return ErrServer
	}
	ownerIDFloat, ok := ownerIDRaw.(float64)
	if !ok {
		slog.ErrorContext(ctx,
			"invalid user id claim type",
			slog.Any("claim_id", ownerIDRaw),
		)
		return ErrServer
	}

	var tx *sql.Tx
	var err error
	if tx, err = DB.BeginTx(ctx, nil); err != nil {
		slog.ErrorContext(ctx,
			"error initializing database transaction",
			slog.String("error", err.Error()),
			slog.Any("user_id", claims["id"]),
		)
		return ErrServer
	}

	defer tx.Rollback()

	var id int64
	if err = tx.QueryRowContext(ctx,
		`
		INSERT INTO offerings (name, owner_id, end_date, description)
		VALUES ($1, $2, $3, $4)
		RETURNING id
		`, req.Name, int(ownerIDFloat), req.EndDate, req.Description,
	).Scan(&id); err != nil {
		slog.ErrorContext(ctx,
			"error inserting new offering on the database",
			slog.String("error", err.Error()),
			slog.Any("user_id", claims["id"]),
		)
		return ErrServer
	}

	enrollmentCode := IDToCode(id)

	if _, err = tx.ExecContext(ctx,
		"UPDATE offerings SET enrollment_code = $1 WHERE id = $2",
		enrollmentCode, id,
	); err != nil {
		slog.ErrorContext(ctx,
			"error updating offering enrollment_code",
			slog.String("error", err.Error()),
			slog.Any("user_id", claims["id"]),
			slog.Int64("offering_id", id),
			slog.String("enrollment_code", enrollmentCode),
		)
		return ErrServer
	}

	if err := tx.Commit(); err != nil {
		slog.ErrorContext(ctx,
			"error committing database transaction",
			slog.String("error", err.Error()),
			slog.Any("user_id", claims["id"]),
		)
		return ErrServer
	}

	return nil
}

const (
	alphabet string = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	base     int64  = int64(len(alphabet))      // 36
	space    int64  = base * base * base * base // 36^4 = 1,679,616
	prime    int64  = 1_276_043                 // Coprime with space (36^4 = 2^8 * 3^8, any prime ≠ 2,3 works)
)

/*
encode converts a non-negative integer "n" into a fixed-width 4-character
string using the constant "alphabet". It is the inverse of the base-36
positional notation, right-padded with the zero character ('A').

n must be in [0, space). Behaviour is undefined outside this range.
*/
func encode(n int64) string {
	digits := make([]byte, 4)
	for i := 3; i >= 0; i-- {
		digits[i] = alphabet[n%base]
		n /= base
	}
	return string(digits)
}

/*
IDToCode converts a unique offering ID into a 4-character enrollment code.

The mapping is deterministic and collision-free: distinct IDs always produce
distinct codes. This is achieved through a multiplicative permutation —
multiplying by a prime coprime with the code space produces a bijection
over [0, space), so no uniqueness check against the database is needed.

The resulting codes appear non-sequential, making it harder for students to
guess or enumerate codes for other class offerings.

id must be in [0, 1,679,615]. If your dataset may exceed this range,
increase the code length or expand the alphabet before deploying.
*/
func IDToCode(id int64) string {
	scrambled := (id * prime) % space
	return encode(scrambled)
}
