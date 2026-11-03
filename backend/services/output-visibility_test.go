package services

import (
	"context"
	"testing"

	"github.com/runcodes-icmc/runcodes/storage"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestOutputFileTestCaseID(t *testing.T) {
	tests := []struct {
		path string
		want int64
	}{
		{"test_1/result.txt", 1},
		{"test_42/nested/dir/out.csv", 42},
		{"outputfiles/explicit.txt", 0},
		{"produced.txt", 0},
		{"test_/result.txt", 0},
		{"test_abc/result.txt", 0},
		{"test_0/result.txt", 0},
		{"other/1/result.txt", 0},
		{"", 0},
	}

	for _, tt := range tests {
		if got := OutputFileTestCaseID(tt.path); got != tt.want {
			t.Errorf("OutputFileTestCaseID(%q) = %d, want %d", tt.path, got, tt.want)
		}
	}
}

func TestOutputFileVisible(t *testing.T) {
	hidden := map[int64]struct{}{3: {}, 7: {}}

	visible := []string{"test_1/result.txt", "outputfiles/explicit.txt", "produced.txt"}
	for _, path := range visible {
		if !OutputFileVisible(path, hidden) {
			t.Errorf("OutputFileVisible(%q) = false, want true", path)
		}
	}

	hiddenPaths := []string{"test_3/result.txt", "test_7/nested/out.csv"}
	for _, path := range hiddenPaths {
		if OutputFileVisible(path, hidden) {
			t.Errorf("OutputFileVisible(%q) = true, want false", path)
		}
	}
}

func TestVisibleOutputFiles(t *testing.T) {
	files := []storage.OutputFile{
		{Path: "test_1/result.txt"},
		{Path: "test_3/secret.txt"},
		{Path: "outputfiles/explicit.txt"},
		{Path: "test_7/nested/out.csv"},
	}

	got := VisibleOutputFiles(files, map[int64]struct{}{3: {}, 7: {}})
	want := []string{"test_1/result.txt", "outputfiles/explicit.txt"}
	if len(got) != len(want) {
		t.Fatalf("files = %v, want %v", got, want)
	}
	for i, path := range want {
		if got[i].Path != path {
			t.Fatalf("files[%d] = %q, want %q", i, got[i].Path, path)
		}
	}

	// A commit with no hidden case keeps the listing untouched.
	all := VisibleOutputFiles(files, nil)
	if len(all) != len(files) {
		t.Fatalf("files with no hidden case = %v, want all", all)
	}
}

func TestMergeCaseIDs(t *testing.T) {
	merged := MergeCaseIDs(
		map[int64]struct{}{1: {}, 2: {}},
		nil,
		map[int64]struct{}{2: {}, 3: {}},
	)

	if len(merged) != 3 {
		t.Fatalf("merged = %v, want three cases", merged)
	}
	for _, caseID := range []int64{1, 2, 3} {
		if _, ok := merged[caseID]; !ok {
			t.Errorf("merged is missing case %d", caseID)
		}
	}
}

func TestInterruptedTestCaseIDs(t *testing.T) {
	mock := newMockDB(t)

	mock.ExpectQuery(`FROM commits_exercise_test_cases_results r`).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"exercise_test_case_id"}).
			AddRow(int64(18)))

	interrupted, err := InterruptedTestCaseIDs(context.Background(), 42)
	if err != nil {
		t.Fatalf("InterruptedTestCaseIDs: %v", err)
	}
	if _, ok := interrupted[18]; !ok {
		t.Fatalf("interrupted = %v, want case 18", interrupted)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
