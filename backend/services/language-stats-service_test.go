package services

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/runcodes-icmc/runcodes/models"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

var languageStatisticsColumns = []string{
	"id", "name", "extension", "is_compilable", "is_available",
	"exercises_offered", "offerings_offered", "professors_offering",
	"submissions", "students", "exercises_chosen",
	"pick_rate_samples", "pick_rate",
	"results", "test_cases",
	"compared_test_cases", "compared_exercises",
	"cpu_median_s", "relative_cpu", "relative_cpu_p25", "relative_cpu_p75",
	"total_exercises", "total_professors",
	"classified_submissions", "classifying_students", "unclassified_submissions",
}

func TestBuildLanguageStatistics(t *testing.T) {
	mock := newMockDB(t)
	mock.ExpectQuery(`WITH catalog`).
		WithArgs(minComparisonLanguages, nil).
		WillReturnRows(sqlmock.NewRows(languageStatisticsColumns).
			AddRow(
				int64(1), "C", "c", true, true,
				int64(4), int64(2), int64(3),
				int64(10), int64(5), int64(3),
				int64(4), 0.25,
				int64(100), int64(12),
				int64(8), int64(3),
				0.0423456, 1.23456, 0.9, 1.5,
				int64(10), int64(5),
				int64(20), int64(7), int64(2),
			).
			AddRow(
				int64(2), "Python 3", "py", false, true,
				int64(0), int64(0), int64(0),
				int64(0), int64(0), int64(0),
				int64(0), nil,
				int64(0), int64(0),
				int64(0), int64(0),
				nil, nil, nil, nil,
				int64(10), int64(5),
				int64(20), int64(7), int64(2),
			))

	statistics, err := buildLanguageStatistics(context.Background(), nil)
	if err != nil {
		t.Fatalf("buildLanguageStatistics: %v", err)
	}

	if statistics.GeneratedAt.IsZero() {
		t.Fatal("generated_at was not set")
	}

	summary := statistics.Summary
	if summary.Exercises != 10 || summary.Professors != 5 {
		t.Fatalf("summary denominators = %+v, want 10 exercises and 5 professors", summary)
	}
	if summary.ClassifiedSubmissions != 20 || summary.Students != 7 {
		t.Fatalf("summary choice totals = %+v, want 20 submissions and 7 students", summary)
	}
	if summary.UnclassifiedSubmissions != 2 {
		t.Fatalf("unclassified = %d, want 2", summary.UnclassifiedSubmissions)
	}
	if summary.CatalogLanguages != 2 || summary.LanguagesOffered != 1 ||
		summary.LanguagesUsed != 1 {
		t.Fatalf("summary language counts = %+v", summary)
	}

	if len(statistics.Languages) != 2 {
		t.Fatalf("got %d languages, want 2", len(statistics.Languages))
	}

	c := statistics.Languages[0]
	if c.Name != "C" || c.Extension != "c" {
		t.Fatalf("first language = %+v, want C/c", c)
	}
	if c.SubmissionShare != 50 {
		t.Errorf("submission share = %v, want 50", c.SubmissionShare)
	}
	if c.ExerciseOfferRate != 40 {
		t.Errorf("exercise offer rate = %v, want 40", c.ExerciseOfferRate)
	}
	if c.ProfessorOfferRate != 60 {
		t.Errorf("professor offer rate = %v, want 60", c.ProfessorOfferRate)
	}
	if c.PickRate == nil || *c.PickRate != 25 {
		t.Errorf("pick rate = %v, want 25", c.PickRate)
	}
	if c.PickRateSamples != 4 {
		t.Errorf("pick rate samples = %d, want 4", c.PickRateSamples)
	}
	if c.CPUTimeMedianSeconds == nil || *c.CPUTimeMedianSeconds != 0.042346 {
		t.Errorf("cpu median = %v, want 0.042346", c.CPUTimeMedianSeconds)
	}
	if c.RelativeCPU == nil || *c.RelativeCPU != 1.235 {
		t.Errorf("relative cpu = %v, want 1.235", c.RelativeCPU)
	}
	if c.RelativeCPUP25 == nil || *c.RelativeCPUP25 != 0.9 {
		t.Errorf("relative cpu p25 = %v, want 0.9", c.RelativeCPUP25)
	}
	if c.RelativeCPUP75 == nil || *c.RelativeCPUP75 != 1.5 {
		t.Errorf("relative cpu p75 = %v, want 1.5", c.RelativeCPUP75)
	}

	py := statistics.Languages[1]
	if py.PickRate != nil || py.RelativeCPU != nil || py.CPUTimeMedianSeconds != nil {
		t.Errorf("empty language has values: %+v", py)
	}
	if py.SubmissionShare != 0 || py.ExerciseOfferRate != 0 {
		t.Errorf("empty language rates = %+v, want zero", py)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestBuildLanguageStatisticsWithOfferingScope(t *testing.T) {
	mock := newMockDB(t)
	mock.ExpectQuery(`scoped_exercises`).
		WithArgs(minComparisonLanguages, int64(3)).
		WillReturnRows(sqlmock.NewRows(languageStatisticsColumns).
			AddRow(
				int64(1), "C", "c", true, true,
				int64(2), int64(1), int64(1),
				int64(4), int64(3), int64(2),
				int64(2), 0.5,
				int64(12), int64(4),
				int64(4), int64(2),
				0.010, 0.9, 0.8, 1.1,
				int64(2), int64(1),
				int64(4), int64(3), int64(0),
			))

	offeringID := int64(3)
	statistics, err := buildLanguageStatistics(context.Background(), &offeringID)
	if err != nil {
		t.Fatalf("buildLanguageStatistics: %v", err)
	}

	if statistics.Summary.Exercises != 2 || statistics.Summary.Professors != 1 {
		t.Fatalf("summary = %+v, want the class-scoped denominators", statistics.Summary)
	}
	if len(statistics.Languages) != 1 {
		t.Fatalf("got %d languages, want 1", len(statistics.Languages))
	}
	if statistics.Languages[0].ExerciseOfferRate != 100 {
		t.Errorf("exercise offer rate = %v, want 100", statistics.Languages[0].ExerciseOfferRate)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestBuildLanguageStatisticsEmptyCatalog(t *testing.T) {
	mock := newMockDB(t)
	mock.ExpectQuery(`WITH catalog`).
		WithArgs(minComparisonLanguages, nil).
		WillReturnRows(sqlmock.NewRows(languageStatisticsColumns))

	statistics, err := buildLanguageStatistics(context.Background(), nil)
	if err != nil {
		t.Fatalf("buildLanguageStatistics: %v", err)
	}
	if statistics.Languages == nil || len(statistics.Languages) != 0 {
		t.Fatalf("languages = %#v, want an empty slice", statistics.Languages)
	}
	if statistics.Summary != (models.LanguageStatisticsSummary{}) {
		t.Fatalf("summary = %+v, want zero", statistics.Summary)
	}
}

func TestBuildLanguageStatisticsDatabaseFailure(t *testing.T) {
	mock := newMockDB(t)
	mock.ExpectQuery(`WITH catalog`).
		WithArgs(minComparisonLanguages, nil).
		WillReturnError(sql.ErrConnDone)

	_, err := buildLanguageStatistics(context.Background(), nil)
	if !errors.Is(err, ErrServer) {
		t.Fatalf("err = %v, want ErrServer", err)
	}
}

func TestPercentHelpers(t *testing.T) {
	if got := percentOf(1, 3); got != 33.33 {
		t.Errorf("percentOf(1, 3) = %v, want 33.33", got)
	}
	if got := percentOf(1, 0); got != 0 {
		t.Errorf("percentOf(1, 0) = %v, want 0", got)
	}

	percent := percentOrNil(sql.NullFloat64{Float64: 0.3333, Valid: true})
	if percent == nil || *percent != 33.33 {
		t.Errorf("percentOrNil = %v, want 33.33", percent)
	}
	if percent := percentOrNil(sql.NullFloat64{}); percent != nil {
		t.Errorf("percentOrNil(NULL) = %v, want nil", percent)
	}

	ratio := roundedOrNil(sql.NullFloat64{Float64: 1.23456, Valid: true}, 3)
	if ratio == nil || *ratio != 1.235 {
		t.Errorf("roundedOrNil = %v, want 1.235", ratio)
	}
	if ratio := roundedOrNil(sql.NullFloat64{}, 3); ratio != nil {
		t.Errorf("roundedOrNil(NULL) = %v, want nil", ratio)
	}
}
