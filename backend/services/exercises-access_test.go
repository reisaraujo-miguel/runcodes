package services

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

// expectExerciseAccess queues the single query loadExerciseAccess runs, returning
// the exercise's offering owner and whether the caller is enrolled in it.
func expectExerciseAccess(mock sqlmock.Sqlmock, ownerID int64, enrolled bool) {
	rows := sqlmock.NewRows([]string{
		"id", "offering_id", "title", "description", "deadline", "open_date",
		"show_before_open_date", "removed", "created_at", "updated_at",
		"ghost", "real_id", "owner_id", "enrolled",
	}).AddRow(
		int64(10), int64(3), "Exercício", "", time.Now().Add(time.Hour),
		time.Now().Add(-time.Hour), false, false, time.Now(), time.Now(),
		false, nil, ownerID, enrolled,
	)

	mock.ExpectQuery(`FROM exercises e`).WillReturnRows(rows)
}

// TestRequireExerciseOwnerEnforcesOwnership is the authorization boundary for
// every authoring mutation (exercises, test cases, compilation and attached
// files): it must admit only the offering's owner, never an enrolled student or a
// professor who merely belongs to the platform.
func TestRequireExerciseOwnerEnforcesOwnership(t *testing.T) {
	const ownerID = 7

	claims := func(userID int64, role string) map[string]any {
		return map[string]any{"id": float64(userID), "role": role}
	}

	tests := []struct {
		name    string
		claims  map[string]any
		enroll  bool
		wantErr error
		wantOwn bool
	}{
		{"professor owning the offering", claims(ownerID, "professor"), false, nil, true},
		{"admin owning the offering", claims(ownerID, "admin"), false, nil, true},
		{"another professor", claims(ownerID+1, "professor"), false, ErrNotOwner, false},
		{"another admin", claims(ownerID+1, "admin"), false, ErrNotOwner, false},
		{"enrolled student", claims(ownerID+1, "student"), true, ErrNotOwner, false},
		{"unrelated user", claims(ownerID+1, "student"), false, ErrNotOwner, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mock := newMockDB(t)
			expectExerciseAccess(mock, ownerID, tc.enroll)

			acc, err := requireExerciseOwner(context.Background(), 10, tc.claims)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("requireExerciseOwner = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr == nil {
				if acc == nil || !acc.isOwner {
					t.Fatalf("expected an owned exercise, got %+v", acc)
				}
				if acc.exercise.ID != 10 {
					t.Fatalf("exercise id = %d, want 10", acc.exercise.ID)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// TestRequireExerciseOwnerErrors covers the failure modes that decide the HTTP
// status: a missing exercise is a 404 and a database failure is a 500, and neither
// may be reported as a permission problem.
func TestRequireExerciseOwnerErrors(t *testing.T) {
	t.Run("missing exercise", func(t *testing.T) {
		mock := newMockDB(t)
		mock.ExpectQuery(`FROM exercises e`).WillReturnError(sql.ErrNoRows)

		_, err := requireExerciseOwner(context.Background(), 10,
			map[string]any{"id": float64(1), "role": "professor"})
		if !errors.Is(err, ErrExerciseNotFound) {
			t.Fatalf("err = %v, want ErrExerciseNotFound", err)
		}
	})

	t.Run("database failure", func(t *testing.T) {
		mock := newMockDB(t)
		mock.ExpectQuery(`FROM exercises e`).WillReturnError(sql.ErrConnDone)

		_, err := requireExerciseOwner(context.Background(), 10,
			map[string]any{"id": float64(1), "role": "professor"})
		if !errors.Is(err, ErrServer) {
			t.Fatalf("err = %v, want ErrServer", err)
		}
	})

	t.Run("malformed claims", func(t *testing.T) {
		// A missing id claim is an internal error, never an authorization pass.
		if _, err := requireExerciseOwner(context.Background(), 10,
			map[string]any{"role": "professor"}); !errors.Is(err, ErrServer) {
			t.Fatalf("err = %v, want ErrServer", err)
		}
	})
}

// TestAuthoringExerciseIDResolvesGhosts pins the indirection the judge also
// applies: authoring rows (test cases, compilation files) belong to the real
// exercise of a ghost, not to the ghost itself.
func TestAuthoringExerciseIDResolvesGhosts(t *testing.T) {
	ghost := &exerciseAccess{ghost: true, realID: sql.NullInt64{Int64: 42, Valid: true}}
	ghost.exercise.ID = 10
	if got := ghost.authoringExerciseID(); got != 42 {
		t.Fatalf("authoringExerciseID = %d, want 42", got)
	}

	plain := &exerciseAccess{}
	plain.exercise.ID = 11
	if got := plain.authoringExerciseID(); got != 11 {
		t.Fatalf("authoringExerciseID = %d, want 11", got)
	}

	// A ghost without a resolvable real id falls back to its own rows rather than
	// to exercise 0.
	unresolved := &exerciseAccess{ghost: true}
	unresolved.exercise.ID = 12
	if got := unresolved.authoringExerciseID(); got != 12 {
		t.Fatalf("authoringExerciseID = %d, want 12", got)
	}
}
