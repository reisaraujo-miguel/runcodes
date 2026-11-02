package services

import (
	"context"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestLoadLanguageMemory(t *testing.T) {
	mock := newMockDB(t)
	mock.ExpectQuery(`mem_usage`).
		WithArgs(minComparisonLanguages, nil).
		WillReturnRows(sqlmock.NewRows(languageMemoryColumns).
			AddRow(
				int64(1), int64(6), int64(2),
				12345678.0, 0.9, 0.8, 1.1,
				int64(20), int64(25),
			).
			AddRow(
				int64(2), int64(0), int64(0),
				5000.0, nil, nil, nil,
				int64(4), int64(10),
			))

	memory, err := loadLanguageMemory(context.Background(), nil)
	if err != nil {
		t.Fatalf("loadLanguageMemory: %v", err)
	}
	entry := memory[1]
	if entry == nil {
		t.Fatal("memory for file type 1 is missing")
	}
	if entry.Relative == nil || *entry.Relative != 0.9 {
		t.Errorf("relative = %v, want 0.9", entry.Relative)
	}
	if entry.MedianBytes == nil || *entry.MedianBytes != 12345678 {
		t.Errorf("median bytes = %v, want 12345678", entry.MedianBytes)
	}
	if entry.Coverage != 80 {
		t.Errorf("coverage = %v, want 80", entry.Coverage)
	}
	if entry.ComparedTestCases != 6 || entry.ComparedExercises != 2 {
		t.Errorf("comparison counts = %+v", entry)
	}

	noComparison := memory[2]
	if noComparison == nil || noComparison.Relative != nil {
		t.Errorf("no-comparison memory = %+v, want a nil relative index", noComparison)
	}
	if noComparison.MedianBytes == nil || *noComparison.MedianBytes != 5000 {
		t.Errorf("no-comparison median = %v, want 5000", noComparison.MedianBytes)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLoadLanguageMemoryDatabaseFailure(t *testing.T) {
	mock := newMockDB(t)
	mock.ExpectQuery(`mem_usage`).
		WithArgs(minComparisonLanguages, nil).
		WillReturnError(context.DeadlineExceeded)

	_, err := loadLanguageMemory(context.Background(), nil)
	if err != ErrServer {
		t.Fatalf("err = %v, want ErrServer", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
