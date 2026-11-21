//go:build integration

package services

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/runcodes-icmc/runcodes/config"
	"github.com/runcodes-icmc/runcodes/database"
)

/*
This file holds the tests that need the real PostgreSQL schema, because the
behaviour they pin is SQL semantics that a mock cannot express: the
(created_at, id) tie-break of the submission numbering, the `s3_key` filter, the
position of an in-flight commit, and the cascade that removes a test case from a
commit's limit snapshot.

Run them with a disposable database that has the schema applied:

	RUNCODES_TEST_DB_DSN='postgres://runcodes:...@localhost:5432/runcodes?sslmode=disable' \
	  go test -tags integration -run 'TestNumberingIntegration|TestFrozenCaseNumberIntegration' ./services
*/
func integrationDB(t *testing.T) *sql.DB {
	t.Helper()

	dsn := os.Getenv("RUNCODES_TEST_DB_DSN")
	if dsn == "" {
		t.Skip("RUNCODES_TEST_DB_DSN is not set")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open the integration database: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("reach the integration database: %v", err)
	}
	db.SetMaxOpenConns(4)

	previousDB := database.DB
	previousConfig := config.C
	database.DB = db
	// The numbering and visibility services only read the platform limits,
	// which SnapshotCommitLimits takes from the configuration.
	config.C = &config.Config{}
	t.Cleanup(func() {
		database.DB = previousDB
		config.C = previousConfig
		db.Close()
	})

	return db
}

// integrationAccount creates a throwaway user, class and exercise, and removes
// them again (the cascade drops the commits, cases and limits with them).
func integrationAccount(t *testing.T, db *sql.DB, exerciseTitle string) (userID, offeringID, exerciseID int64) {
	t.Helper()

	suffix := time.Now().UnixNano()
	email := fmt.Sprintf("numbering-itest-%d@invalid", suffix)

	if err := db.QueryRow(`INSERT INTO users (name, email, password_hash)
		VALUES ($1, $2, '!') RETURNING id`,
		"numbering itest", email,
	).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM users WHERE id = $1`, userID)
	})

	if err := db.QueryRow(`INSERT INTO offerings (name, owner_id, end_date)
		VALUES ($1, $2, now() + interval '30 days') RETURNING id`,
		fmt.Sprintf("numbering itest %d", suffix), userID,
	).Scan(&offeringID); err != nil {
		t.Fatalf("insert offering: %v", err)
	}

	if err := db.QueryRow(`INSERT INTO exercises
		    (offering_id, creator_id, title, deadline, open_date)
		VALUES ($1, $2, $3, now() + interval '30 days', now() - interval '1 day')
		RETURNING id`,
		offeringID, userID, exerciseTitle,
	).Scan(&exerciseID); err != nil {
		t.Fatalf("insert exercise: %v", err)
	}

	return userID, offeringID, exerciseID
}

func integrationCase(t *testing.T, db *sql.DB, exerciseID int64, name string) int64 {
	t.Helper()

	var caseID int64
	if err := db.QueryRow(`INSERT INTO exercises_test_cases
		    (exercise_id, input, input_type, expected_output, expected_output_type)
		VALUES ($1, $2, 'text', 'ok', 'text')
		RETURNING id`,
		exerciseID, name,
	).Scan(&caseID); err != nil {
		t.Fatalf("insert test case %s: %v", name, err)
	}

	return caseID
}

// integrationCommit inserts one commit at an explicit time. A nil key models a
// submission the judge has not archived yet, which still counts for numbering.
func integrationCommit(
	t *testing.T, db *sql.DB, userID, exerciseID int64, at time.Time, key *string,
) int64 {
	t.Helper()

	var commitID int64
	if err := db.QueryRow(`INSERT INTO commits
		    (user_id, exercise_id, created_at, status, s3_key)
		VALUES ($1, $2, $3, 'completed', $4)
		RETURNING id`,
		userID, exerciseID, at, key,
	).Scan(&commitID); err != nil {
		t.Fatalf("insert commit: %v", err)
	}

	return commitID
}

/*
TestNumberingIntegration pins the rules the history and the snapshot share: the
position is per user and exercise, ties in created_at are broken by id, and an
in-flight submission (no s3_key) still occupies its position even though the
history does not list it.
*/
func TestNumberingIntegration(t *testing.T) {
	db := integrationDB(t)
	ctx := context.Background()

	userID, _, exerciseID := integrationAccount(t, db, "numbering itest")
	otherUserID, _, otherExerciseID := integrationAccount(t, db, "numbering itest other")

	base := time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC)
	// Two commits share an instant: the id breaks the tie in the same direction
	// for the window function and the count.
	first := integrationCommit(t, db, userID, exerciseID, base, ptrString("k1"))
	second := integrationCommit(t, db, userID, exerciseID, base, ptrString("k2"))
	// An in-flight submission has no object key yet.
	inFlight := integrationCommit(t, db, userID, exerciseID, base.Add(time.Hour), nil)
	last := integrationCommit(t, db, userID, exerciseID, base.Add(2*time.Hour), ptrString("k4"))
	// Another user's submission and another exercise's must not move the numbers.
	integrationCommit(t, db, otherUserID, exerciseID, base.Add(3*time.Hour), ptrString("kx"))
	integrationCommit(t, db, userID, otherExerciseID, base.Add(4*time.Hour), ptrString("ky"))

	positions := map[int64]int64{first: 1, second: 2, inFlight: 3, last: 4}
	for commitID, want := range positions {
		got, err := SubmissionNumber(ctx, commitID)
		if err != nil {
			t.Fatalf("SubmissionNumber(%d): %v", commitID, err)
		}
		if got != want {
			t.Errorf("SubmissionNumber(%d) = %d, want %d", commitID, got, want)
		}
	}

	// The history lists only archived submissions, newest first, numbered with
	// the same positions as the count above.
	submissions, err := ListMyExerciseSubmissions(ctx, exerciseID, userID)
	if err != nil {
		t.Fatalf("ListMyExerciseSubmissions: %v", err)
	}
	wantIDs := []int64{last, second, first}
	wantNumbers := []int64{4, 2, 1}
	if len(submissions) != len(wantIDs) {
		t.Fatalf("history = %+v, want %d entries", submissions, len(wantIDs))
	}
	for i := range wantIDs {
		if submissions[i].ID != wantIDs[i] || submissions[i].Number != wantNumbers[i] {
			t.Errorf("history[%d] = (id %d, number %d), want (%d, %d)",
				i, submissions[i].ID, submissions[i].Number, wantIDs[i], wantNumbers[i])
		}
	}
}

/*
TestFrozenCaseNumberIntegration pins the fix for generated-file authorization:
the case number is written with the snapshot and survives the deletion of a test
case, and a legacy `test_<id>/` path is still resolved by id.
*/
func TestFrozenCaseNumberIntegration(t *testing.T) {
	db := integrationDB(t)
	ctx := context.Background()

	userID, _, exerciseID := integrationAccount(t, db, "frozen number itest")
	cases := []int64{
		integrationCase(t, db, exerciseID, "1"),
		integrationCase(t, db, exerciseID, "2"),
		integrationCase(t, db, exerciseID, "3"),
	}

	commitID := integrationCommit(t, db, userID, exerciseID, time.Now().UTC(), ptrString("src"))

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := SnapshotCommitLimits(ctx, tx, []int64{commitID}); err != nil {
		_ = tx.Rollback()
		t.Fatalf("SnapshotCommitLimits: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	index, err := LoadCommitCaseIndex(ctx, commitID)
	if err != nil {
		t.Fatalf("LoadCommitCaseIndex: %v", err)
	}
	for number, caseID := range map[int]int64{1: cases[0], 2: cases[1], 3: cases[2]} {
		if got, ok := index.IDForNumber(number); !ok || got != caseID {
			t.Fatalf("IDForNumber(%d) = (%d, %v), want (%d, true)", number, got, ok, caseID)
		}
	}

	// Deleting the middle case cascades its limit row. The frozen numbers of the
	// surviving cases must not move.
	if _, err := db.ExecContext(ctx,
		`DELETE FROM exercises_test_cases WHERE id = $1`, cases[1],
	); err != nil {
		t.Fatalf("delete the middle case: %v", err)
	}

	index, err = LoadCommitCaseIndex(ctx, commitID)
	if err != nil {
		t.Fatalf("LoadCommitCaseIndex after delete: %v", err)
	}
	if _, ok := index.IDForNumber(2); ok {
		t.Error("the deleted case's number still resolves to a case")
	}
	if got, ok := index.IDForNumber(3); !ok || got != cases[2] {
		t.Errorf("IDForNumber(3) after delete = (%d, %v), want (%d, true)", got, ok, cases[2])
	}

	// A stored file keeps its own case, and a file of the deleted case is not
	// attributed to the case that followed it.
	if got, ok := ResolveOutputFileTestCaseID("test_v2_3/result.txt", index); !ok || got != cases[2] {
		t.Errorf("test_v2_3 resolved to (%d, %v), want (%d, true)", got, ok, cases[2])
	}
	if _, ok := ResolveOutputFileTestCaseID("test_v2_2/result.txt", index); ok {
		t.Error("the deleted case's file still resolves to a case")
	}

	// A legacy object names the database id, not a position.
	if got, ok := ResolveOutputFileTestCaseID(
		fmt.Sprintf("test_%d/result.txt", cases[0]), index,
	); !ok || got != cases[0] {
		t.Errorf("legacy path resolved to (%d, %v), want (%d, true)", got, ok, cases[0])
	}

	// The hidden-output rule follows the resolved case.
	hidden := map[int64]struct{}{cases[2]: {}}
	if OutputFileVisibleForCases("test_v2_3/result.txt", hidden, index) {
		t.Error("a hidden case's current file is still visible")
	}
	if !OutputFileVisibleForCases("test_v2_1/result.txt", hidden, index) {
		t.Error("a visible case's file was hidden")
	}
}
