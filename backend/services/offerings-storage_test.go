package services

import (
	"context"
	"testing"

	"github.com/runcodes-icmc/runcodes/storage"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

// TestOfferingStorageKeysIncludesTextCases pins that deleting a class removes
// text test-case objects too. They were previously skipped because only file
// cases were considered to own an object.
func TestOfferingStorageKeysIncludesTextCases(t *testing.T) {
	mock := newMockDB(t)

	mock.ExpectQuery(`SELECT tc.id, COALESCE\(files.path, ''\)`).
		WithArgs(int64(5)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "path"}).
			AddRow(int64(11), "a.txt").
			AddRow(int64(11), "b.txt"))
	mock.ExpectQuery(`SELECT files.exercise_id, files.path, 'compilation'`).
		WithArgs(int64(5)).
		WillReturnRows(sqlmock.NewRows([]string{"exercise_id", "path", "kind"}))
	mock.ExpectQuery(`SELECT c.s3_key`).
		WithArgs(int64(5)).
		WillReturnRows(sqlmock.NewRows([]string{"s3_key"}))

	keys, err := offeringStorageKeys(context.Background(), 5)
	if err != nil {
		t.Fatalf("offeringStorageKeys: %v", err)
	}

	want := map[string]bool{
		storage.CaseInputKey(11):         false,
		storage.CaseOutputKey(11):        false,
		storage.CaseFileKey(11, "a.txt"): false,
		storage.CaseFileKey(11, "b.txt"): false,
	}
	for _, key := range keys {
		if _, ok := want[key.key]; !ok {
			t.Fatalf("unexpected key %q", key.key)
		}
		if want[key.key] {
			t.Fatalf("duplicate key %q", key.key)
		}
		want[key.key] = true
	}
	for key, seen := range want {
		if !seen {
			t.Errorf("missing key %q", key)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
