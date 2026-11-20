package services

import (
	"context"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestLoadCommitCaseIndex(t *testing.T) {
	mock := newMockDB(t)

	mock.ExpectQuery(`SELECT case_number, exercise_test_case_id`).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"case_number", "exercise_test_case_id"}).
			AddRow(1, int64(7)).
			AddRow(2, int64(9)))

	index, err := LoadCommitCaseIndex(context.Background(), 42)
	if err != nil {
		t.Fatalf("LoadCommitCaseIndex: %v", err)
	}
	if id, ok := index.IDForNumber(1); !ok || id != 7 {
		t.Fatalf("IDForNumber(1) = (%d, %v), want (7, true)", id, ok)
	}
	if id, ok := index.IDForNumber(2); !ok || id != 9 {
		t.Fatalf("IDForNumber(2) = (%d, %v), want (9, true)", id, ok)
	}
	if _, ok := index.IDForNumber(3); ok {
		t.Fatal("IDForNumber(3) resolved a number the snapshot does not have")
	}
	if !index.HasID(7) || index.HasID(8) {
		t.Fatal("HasID does not match the frozen case ids")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLoadCommitCaseIndexWithoutFrozenNumber(t *testing.T) {
	// A commit queued before the numbering was frozen has no snapshot rows.
	// The index is empty, so every case file fails closed.
	mock := newMockDB(t)

	mock.ExpectQuery(`SELECT case_number, exercise_test_case_id`).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"case_number", "exercise_test_case_id"}))

	index, err := LoadCommitCaseIndex(context.Background(), 42)
	if err != nil {
		t.Fatalf("LoadCommitCaseIndex: %v", err)
	}
	if _, ok := index.IDForNumber(1); ok {
		t.Fatal("an empty index resolved a case number")
	}
	if index.HasID(7) {
		t.Fatal("an empty index resolved a case id")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestListCommitArtifacts(t *testing.T) {
	mock := newMockDB(t)

	limit := int64(1024)
	number := int64(2)
	mock.ExpectQuery(`FROM commit_test_case_artifacts`).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows(
			[]string{"exercise_test_case_id", "state", "reason", "limit_bytes", "case_number"},
		).
			AddRow(int64(7), "failed", "commit_artifact_budget_exceeded", limit, number).
			AddRow(int64(9), "ready", nil, nil, nil))

	artifacts, err := ListCommitArtifacts(context.Background(), 42)
	if err != nil {
		t.Fatalf("ListCommitArtifacts: %v", err)
	}
	if len(artifacts) != 2 {
		t.Fatalf("artifacts = %+v, want two", artifacts)
	}
	if artifacts[0].TestCaseID == nil || *artifacts[0].TestCaseID != 7 ||
		artifacts[0].State != "failed" ||
		artifacts[0].Reason != "commit_artifact_budget_exceeded" ||
		artifacts[0].LimitBytes == nil || *artifacts[0].LimitBytes != limit ||
		artifacts[0].Number == nil || *artifacts[0].Number != number {
		t.Fatalf("first artifact = %+v", artifacts[0])
	}
	if artifacts[1].State != "ready" || artifacts[1].Reason != "" ||
		artifacts[1].LimitBytes != nil || artifacts[1].Number != nil {
		t.Fatalf("second artifact = %+v", artifacts[1])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
