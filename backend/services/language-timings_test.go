package services

import (
	"context"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestLoadLanguageTimings(t *testing.T) {
	mock := newMockDB(t)
	mock.ExpectQuery(`commit_phase_timings`).
		WithArgs(nil).
		WillReturnRows(sqlmock.NewRows(languageTimingsColumns).
			AddRow(
				int64(1), "compile", int64(4), int64(0), int64(0),
				1200.0, 1000.0, 1500.0, 900.0, 12000000.0, int64(5),
			).
			AddRow(
				int64(1), "workspace_prepare", int64(5), int64(0), int64(0),
				800.0, 700.0, 900.0, nil, nil, int64(5),
			).
			AddRow(
				int64(1), "image_ensure", int64(5), int64(2), int64(3),
				40.0, 30.0, 50.0, nil, nil, int64(5),
			).
			AddRow(
				int64(1), "container_create", int64(5), int64(0), int64(0),
				45.0, 40.0, 50.0, nil, nil, int64(5),
			).
			AddRow(
				int64(1), "container_start", int64(5), int64(0), int64(0),
				120.0, 100.0, 140.0, nil, nil, int64(5),
			).
			AddRow(
				int64(1), "source_download", int64(5), int64(0), int64(0),
				100.0, 90.0, 110.0, nil, nil, int64(5),
			).
			AddRow(
				int64(2), "startup", int64(1), int64(0), int64(0),
				300.0, 300.0, 300.0, nil, nil, int64(2),
			))

	timings, err := loadLanguageTimings(context.Background(), nil)
	if err != nil {
		t.Fatalf("loadLanguageTimings: %v", err)
	}
	if len(timings) != 2 {
		t.Fatalf("got %d languages, want 2", len(timings))
	}

	entry := timings[1]
	if entry.Compile == nil || entry.Compile.MedianSeconds == nil {
		t.Fatalf("compile timing missing: %+v", entry.Compile)
	}
	if *entry.Compile.MedianSeconds != 1.2 {
		t.Errorf("compile median = %v, want 1.2s", *entry.Compile.MedianSeconds)
	}
	if entry.Compile.CPUTimeMedianSeconds == nil || *entry.Compile.CPUTimeMedianSeconds != 0.9 {
		t.Errorf("compile cpu = %v, want 0.9s", entry.Compile.CPUTimeMedianSeconds)
	}
	if entry.Compile.MemPeakBytes == nil || *entry.Compile.MemPeakBytes != 12000000 {
		t.Errorf("compile memory = %v, want 12000000", entry.Compile.MemPeakBytes)
	}
	if entry.Compile.Coverage != 80 {
		t.Errorf("compile coverage = %v, want 80", entry.Compile.Coverage)
	}

	if entry.Workspace == nil || entry.Workspace.MedianSeconds == nil ||
		*entry.Workspace.MedianSeconds != 0.8 {
		t.Errorf("workspace timing = %+v, want 0.8s", entry.Workspace)
	}

	if entry.Image == nil || entry.Image.MedianSeconds == nil ||
		*entry.Image.MedianSeconds != 0.04 {
		t.Errorf("image timing = %+v, want 0.04s", entry.Image)
	}
	if entry.Image.WarmSamples != 3 || entry.Image.ColdSamples != 2 {
		t.Errorf("image samples = %+v, want 3 warm and 2 cold", entry.Image)
	}

	// The runtime's own calls are part of the report: a slow create/start is an
	// operational problem, and it was previously invisible.
	if entry.ContainerCreate == nil || entry.ContainerCreate.MedianSeconds == nil ||
		*entry.ContainerCreate.MedianSeconds != 0.045 {
		t.Errorf("container_create timing = %+v, want 0.045s", entry.ContainerCreate)
	}
	if entry.ContainerStart == nil || entry.ContainerStart.MedianSeconds == nil ||
		*entry.ContainerStart.MedianSeconds != 0.12 {
		t.Errorf("container_start timing = %+v, want 0.12s", entry.ContainerStart)
	}

	if timings[2].Startup == nil || timings[2].Startup.MedianSeconds == nil ||
		*timings[2].Startup.MedianSeconds != 0.3 {
		t.Errorf("startup timing = %+v, want 0.3s", timings[2].Startup)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLoadLanguageTimingsDatabaseFailure(t *testing.T) {
	mock := newMockDB(t)
	mock.ExpectQuery(`commit_phase_timings`).
		WithArgs(nil).
		WillReturnError(context.DeadlineExceeded)

	_, err := loadLanguageTimings(context.Background(), nil)
	if err != ErrServer {
		t.Fatalf("err = %v, want ErrServer", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
