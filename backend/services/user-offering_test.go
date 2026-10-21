package services

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

// expectUserOffering queues the two queries GetUserOffering runs: the class and
// the caller's relation to it, then the visible exercises with their progress.
func expectUserOffering(
	mock sqlmock.Sqlmock, offeringID, userID int64,
) (lastSubmittedAt time.Time, deadline time.Time) {
	lastSubmittedAt = time.Date(2026, 10, 20, 18, 0, 0, 0, time.UTC)
	deadline = time.Date(2026, 11, 10, 23, 59, 0, 0, time.UTC)
	openedAt := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`FROM offerings o`).
		WithArgs(userID, offeringID).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "description", "end_date", "role", "owner_name",
			"is_owner", "member_count",
		}).AddRow(
			offeringID, "Estruturas de Dados", "Turma de 2026", deadline,
			EnrollmentRoleStudent, "Ada Lovelace", false, int64(12),
		))

	mock.ExpectQuery(`FROM exercises e`).
		WithArgs(offeringID, userID).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "offering_id", "title", "description", "deadline",
			"open_date", "submission_count", "last_submission_at",
			"last_status", "best_score",
		}).
			AddRow(
				int64(11), offeringID, "Lista 1", "Primeira lista", deadline,
				openedAt, int64(2), lastSubmittedAt, "completed", 87.5,
			).
			AddRow(
				int64(12), offeringID, "Lista 2", "", deadline.Add(24*time.Hour),
				openedAt, int64(0), nil, nil, nil,
			))

	return lastSubmittedAt, deadline
}

func TestGetUserOffering(t *testing.T) {
	mock := newMockDB(t)
	lastSubmittedAt, deadline := expectUserOffering(mock, 3, 7)

	detail, err := GetUserOffering(context.Background(), 3,
		map[string]any{"id": float64(7), "role": "student"})
	if err != nil {
		t.Fatalf("GetUserOffering: %v", err)
	}

	if detail.ID != 3 || detail.Name != "Estruturas de Dados" {
		t.Fatalf("offering = %+v", detail)
	}
	if detail.Role != EnrollmentRoleStudent || detail.IsOwner {
		t.Fatalf("role = %q, is_owner = %v", detail.Role, detail.IsOwner)
	}
	if detail.OwnerName != "Ada Lovelace" || detail.MemberCount != 12 {
		t.Fatalf("owner = %q, members = %d", detail.OwnerName, detail.MemberCount)
	}
	if !detail.EndDate.Equal(deadline) {
		t.Fatalf("end date = %v, want %v", detail.EndDate, deadline)
	}

	if len(detail.Exercises) != 2 {
		t.Fatalf("got %d exercises, want 2", len(detail.Exercises))
	}

	first := detail.Exercises[0]
	if first.SubmissionCount != 2 || first.BestScore == nil || *first.BestScore != 87.5 {
		t.Fatalf("first exercise progress = %+v", first)
	}
	if first.LastStatus == nil || *first.LastStatus != "completed" {
		t.Fatalf("last status = %v, want completed", first.LastStatus)
	}
	if first.LastSubmissionAt == nil ||
		!first.LastSubmissionAt.Equal(lastSubmittedAt) {
		t.Fatalf("last submission = %v, want %v",
			first.LastSubmissionAt, lastSubmittedAt)
	}

	second := detail.Exercises[1]
	if second.SubmissionCount != 0 || second.LastStatus != nil ||
		second.LastSubmissionAt != nil || second.BestScore != nil {
		t.Fatalf("exercise without submissions = %+v", second)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGetUserOfferingHidesForeignAndBannedClasses(t *testing.T) {
	// The query itself filters out a non-member and a banned member, so both are
	// indistinguishable from a missing class.
	mock := newMockDB(t)
	mock.ExpectQuery(`FROM offerings o`).
		WithArgs(int64(7), int64(3)).
		WillReturnError(sql.ErrNoRows)

	_, err := GetUserOffering(context.Background(), 3,
		map[string]any{"id": float64(7), "role": "student"})
	if !errors.Is(err, ErrOfferingNotFound) {
		t.Fatalf("err = %v, want ErrOfferingNotFound", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGetUserOfferingExerciseQueryFailure(t *testing.T) {
	mock := newMockDB(t)
	mock.ExpectQuery(`FROM offerings o`).
		WithArgs(int64(7), int64(3)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "description", "end_date", "role", "owner_name",
			"is_owner", "member_count",
		}).AddRow(
			int64(3), "Estruturas de Dados", "", time.Now(),
			EnrollmentRoleStudent, "Ada Lovelace", false, int64(12),
		))
	mock.ExpectQuery(`FROM exercises e`).
		WithArgs(int64(3), int64(7)).
		WillReturnError(sql.ErrConnDone)

	_, err := GetUserOffering(context.Background(), 3,
		map[string]any{"id": float64(7), "role": "student"})
	if !errors.Is(err, ErrServer) {
		t.Fatalf("err = %v, want ErrServer", err)
	}
}

func TestGetUserOfferingRejectsMalformedClaims(t *testing.T) {
	// A missing id claim is an internal error, never an authorization pass.
	if _, err := GetUserOffering(context.Background(), 3,
		map[string]any{"role": "student"}); !errors.Is(err, ErrServer) {
		t.Fatalf("err = %v, want ErrServer", err)
	}
}
