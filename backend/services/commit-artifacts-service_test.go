package services

import (
	"context"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestCommitTestCaseIDs(t *testing.T) {
	mock := newMockDB(t)

	mock.ExpectQuery(`SELECT tc.id`).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).
			AddRow(int64(7)).
			AddRow(int64(9)))

	ids, err := CommitTestCaseIDs(context.Background(), 42)
	if err != nil {
		t.Fatalf("CommitTestCaseIDs: %v", err)
	}
	if len(ids) != 2 || ids[0] != 7 || ids[1] != 9 {
		t.Fatalf("ids = %v, want [7 9]", ids)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestListCommitArtifacts(t *testing.T) {
	mock := newMockDB(t)

	limit := int64(1024)
	mock.ExpectQuery(`FROM commit_test_case_artifacts`).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows(
			[]string{"exercise_test_case_id", "state", "reason", "limit_bytes"},
		).
			AddRow(int64(7), "failed", "commit_artifact_budget_exceeded", limit).
			AddRow(int64(9), "ready", nil, nil))

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
		artifacts[0].LimitBytes == nil || *artifacts[0].LimitBytes != limit {
		t.Fatalf("first artifact = %+v", artifacts[0])
	}
	if artifacts[1].State != "ready" || artifacts[1].Reason != "" ||
		artifacts[1].LimitBytes != nil {
		t.Fatalf("second artifact = %+v", artifacts[1])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
