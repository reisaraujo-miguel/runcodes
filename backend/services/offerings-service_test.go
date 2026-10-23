package services

import (
	"context"
	"testing"
	"time"

	"github.com/runcodes-icmc/runcodes/config"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestRotateOfferingEnrollmentCode(t *testing.T) {
	previousConfig := config.C
	config.C = &config.Config{Redis: config.RedisConfig{Addr: "127.0.0.1:0"}}
	t.Cleanup(func() { config.C = previousConfig })

	mock := newMockDB(t)
	mock.ExpectQuery(`SELECT id, name, end_date, COALESCE\(description, ''\)`).
		WithArgs(int64(3)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "end_date", "description", "enrollment_code",
			"owner_id", "visible_to_enroll",
		}).AddRow(
			int64(3), "Turma", time.Now().Add(time.Hour), "", "OLD123",
			int64(7), true,
		))
	mock.ExpectExec(`UPDATE offerings`).
		WithArgs(sqlmock.AnyArg(), int64(3)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	offering, err := RotateOfferingEnrollmentCode(
		context.Background(), 3,
		map[string]any{"id": float64(7), "role": "professor"},
	)
	if err != nil {
		t.Fatalf("RotateOfferingEnrollmentCode: %v", err)
	}
	if len(offering.EnrollmentCode) != codeLen {
		t.Fatalf("code = %q, want %d characters", offering.EnrollmentCode, codeLen)
	}
	if offering.EnrollmentCode == "OLD123" {
		t.Fatal("rotation returned the old code")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
