package services

import (
	"context"
	"database/sql"
	"log/slog"
	"math"

	"github.com/runcodes-icmc/runcodes/cache"
	"github.com/runcodes-icmc/runcodes/database"
	"github.com/runcodes-icmc/runcodes/models"
)

// minComparisonLanguages is how many languages a test case must have before it
// enters the CPU comparison. A case with a single language has no peer; counting
// it would award that language a 1.0 ratio for being the only option on many
// exercises, which is exactly the bias the relative index is meant to remove.
const minComparisonLanguages = 2

/*
GetLanguageStatistics returns the platform-wide language report, cached for a
short interval because the query walks every executed test case.

The report answers three different questions, and keeping their denominators
separate is what makes the comparison fair:

  - Choice: student submissions per catalog entry, and the share each entry got
    inside the exercises that allowed it (so a language offered in fewer
    exercises is not penalised for the smaller opportunity).
  - Allowance: distinct exercises, classes and professors that allow the entry.
  - CPU: for every test case, the entry's median CPU time divided by the median
    of the languages that ran that same test case. The per-entry index is the
    median of those ratios over comparable test cases, which removes exercise
    difficulty and per-case limits from the comparison.
*/
func GetLanguageStatistics(ctx context.Context) (*models.LanguageStatistics, error) {
	var cached models.LanguageStatistics
	if cache.GetJSON(ctx, cache.KeyLanguageStatistics, &cached) {
		return &cached, nil
	}

	statistics, err := buildLanguageStatistics(ctx, nil)
	if err != nil {
		return nil, err
	}

	cache.SetJSON(ctx, cache.KeyLanguageStatistics, statistics,
		cache.LanguageStatisticsTTL)

	return statistics, nil
}

/*
GetOfferingLanguageStatistics returns the same report scoped to one class. Only
the class owner, a co-professor or the admin panel may read it: the report
contains every student's choice and performance in that class.

The cache key is per class. The platform report and a class report share the
query, metrics and fairness rules; only the exercise scope changes.
*/
func GetOfferingLanguageStatistics(
	ctx context.Context, offeringID int64, claims map[string]any,
) (*models.LanguageStatistics, error) {
	userID, role, err := claimsUserID(claims)
	if err != nil {
		return nil, err
	}

	rel, err := loadOfferingRelation(ctx, offeringID, userID, role)
	if err != nil {
		return nil, err
	}
	if !rel.CanAuthor() {
		// A student or monitor must not read the class-wide adoption report.
		// Treat the class as missing, like every other author-only view.
		return nil, ErrOfferingNotFound
	}

	key := cache.OfferingLanguageStatisticsKey(offeringID)
	var cached models.LanguageStatistics
	if cache.GetJSON(ctx, key, &cached) {
		return &cached, nil
	}

	statistics, err := buildLanguageStatistics(ctx, &offeringID)
	if err != nil {
		return nil, err
	}

	cache.SetJSON(ctx, key, statistics, cache.LanguageStatisticsTTL)

	return statistics, nil
}

/*
buildLanguageStatistics runs the aggregation without the cache. It is the
implementation of both report variants and the seam their tests use, so a test
does not need Redis or the process configuration.

A nil offeringID means the whole platform; a non-nil id scopes every
denominator to that class's non-removed exercises.
*/
func buildLanguageStatistics(
	ctx context.Context, offeringID *int64,
) (*models.LanguageStatistics, error) {
	// database/sql writes the untyped nil as SQL NULL, which the query treats as
	// "no scope filter".
	var scope any
	if offeringID != nil {
		scope = *offeringID
	}

	rows, err := database.DB.QueryContext(ctx, languageStatisticsQuery,
		minComparisonLanguages, scope,
	)
	if err != nil {
		slog.ErrorContext(ctx, "error loading language statistics",
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	defer rows.Close()

	statistics := &models.LanguageStatistics{
		GeneratedAt: nowFunc().UTC(),
		Languages:   make([]models.LanguageStat, 0, 24),
	}

	for rows.Next() {
		var (
			entry                 models.LanguageStat
			exercisesOffered      int64
			offeringsOffered      int64
			professorsOffering    int64
			pickRate              sql.NullFloat64
			pickRateSamples       int64
			results               int64
			testCases             int64
			comparedTestCases     int64
			comparedExercises     int64
			cpuMedianSeconds      sql.NullFloat64
			relativeCPU           sql.NullFloat64
			relativeCPUP25        sql.NullFloat64
			relativeCPUP75        sql.NullFloat64
			totalExercises        int64
			totalProfessors       int64
			classifiedSubmissions int64
			classifyingStudents   int64
			unclassified          int64
		)

		if err := rows.Scan(
			&entry.FileTypeID, &entry.Name, &entry.Extension,
			&entry.IsCompilable, &entry.IsAvailable,
			&exercisesOffered, &offeringsOffered, &professorsOffering,
			&entry.Submissions, &entry.Students, &entry.ExercisesChosen,
			&pickRateSamples, &pickRate,
			&results, &testCases,
			&comparedTestCases, &comparedExercises,
			&cpuMedianSeconds, &relativeCPU, &relativeCPUP25, &relativeCPUP75,
			&totalExercises, &totalProfessors,
			&classifiedSubmissions, &classifyingStudents,
			&unclassified,
		); err != nil {
			slog.ErrorContext(ctx, "error scanning language statistics",
				slog.String("error", err.Error()),
			)
			return nil, ErrServer
		}

		entry.ExercisesOffered = exercisesOffered
		entry.OfferingsOffered = offeringsOffered
		entry.ProfessorsOffering = professorsOffering
		entry.PickRateSamples = pickRateSamples
		entry.Results = results
		entry.TestCases = testCases
		entry.ComparedTestCases = comparedTestCases
		entry.ComparedExercises = comparedExercises

		entry.PickRate = percentOrNil(pickRate)
		entry.CPUTimeMedianSeconds = roundedOrNil(cpuMedianSeconds, 6)
		entry.RelativeCPU = roundedOrNil(relativeCPU, 3)
		entry.RelativeCPUP25 = roundedOrNil(relativeCPUP25, 3)
		entry.RelativeCPUP75 = roundedOrNil(relativeCPUP75, 3)

		statistics.Summary.Exercises = totalExercises
		statistics.Summary.Professors = totalProfessors
		statistics.Summary.ClassifiedSubmissions = classifiedSubmissions
		statistics.Summary.Students = classifyingStudents
		statistics.Summary.UnclassifiedSubmissions = unclassified

		statistics.Languages = append(statistics.Languages, entry)
	}
	if err := rows.Err(); err != nil {
		slog.ErrorContext(ctx, "error iterating language statistics",
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}

	statistics.Summary.CatalogLanguages = int64(len(statistics.Languages))
	for i := range statistics.Languages {
		entry := &statistics.Languages[i]

		entry.SubmissionShare = percentOf(
			entry.Submissions, statistics.Summary.ClassifiedSubmissions,
		)
		entry.ExerciseOfferRate = percentOf(
			entry.ExercisesOffered, statistics.Summary.Exercises,
		)
		entry.ProfessorOfferRate = percentOf(
			entry.ProfessorsOffering, statistics.Summary.Professors,
		)

		if entry.ExercisesOffered > 0 {
			statistics.Summary.LanguagesOffered++
		}
		if entry.Submissions > 0 {
			statistics.Summary.LanguagesUsed++
		}
	}

	if len(statistics.Languages) == 0 {
		return statistics, nil
	}

	timings, err := loadLanguageTimings(ctx, offeringID)
	if err != nil {
		return nil, err
	}
	for i := range statistics.Languages {
		entry := &statistics.Languages[i]
		if measured, ok := timings[entry.FileTypeID]; ok {
			entry.Timings = measured
		}
	}

	return statistics, nil
}

/*
languageStatisticsQuery aggregates the platform or one class in one round trip.
The second parameter is the offering scope: NULL means every class, an id
restricts `scoped_exercises` and therefore every denominator. Every language row
carries the totals it is compared against, so the service only has to turn counts
into percentages.

The CPU part is deliberately paired: `per_case` reduces a language to one median
per test case, `per_case_baseline` computes the median across the languages on
that same case, and `ratios` divides the two. Aggregating the ratios per language
with a median (and a quartile range) keeps a noisy high-usage language from
dominating the result.
*/
const languageStatisticsQuery = `
WITH catalog AS (
  SELECT id, name, extension, is_compilable, is_available
  FROM allowed_file_types
),
scoped_exercises AS (
  SELECT id, offering_id, creator_id
  FROM exercises
  WHERE NOT removed
    AND ($2::int IS NULL OR offering_id = $2)
),
totals AS (
  SELECT
    (SELECT count(*) FROM scoped_exercises) AS exercises,
    (SELECT count(DISTINCT creator_id) FROM scoped_exercises) AS professors,
    (SELECT count(*)
       FROM commits c
       JOIN users u ON u.id = c.user_id
       JOIN scoped_exercises e ON e.id = c.exercise_id
      WHERE u.role = 'student'
        AND c.allowed_file_type_id IS NULL) AS unclassified_submissions
),
allowance AS (
  SELECT eaft.allowed_file_type_id AS file_type_id,
         count(DISTINCT e.id) AS exercises_offered,
         count(DISTINCT e.offering_id) AS offerings_offered,
         count(DISTINCT e.creator_id) AS professors_offering
  FROM scoped_exercises e
  JOIN exercises_allowed_file_types eaft ON eaft.exercise_id = e.id
  GROUP BY eaft.allowed_file_type_id
),
student_commits AS (
  SELECT c.exercise_id, c.allowed_file_type_id, c.user_id
  FROM commits c
  JOIN users u ON u.id = c.user_id
  JOIN scoped_exercises e ON e.id = c.exercise_id
  WHERE u.role = 'student'
    AND c.allowed_file_type_id IS NOT NULL
),
choice AS (
  SELECT allowed_file_type_id AS file_type_id,
         count(*) AS submissions,
         count(DISTINCT user_id) AS students,
         count(DISTINCT exercise_id) AS exercises_chosen
  FROM student_commits
  GROUP BY allowed_file_type_id
),
choice_totals AS (
  SELECT count(*) AS submissions,
         count(DISTINCT user_id) AS students
  FROM student_commits
),
exercise_totals AS (
  SELECT exercise_id, count(*) AS submissions
  FROM student_commits
  GROUP BY exercise_id
),
per_exercise_type AS (
  SELECT exercise_id, allowed_file_type_id AS file_type_id, count(*) AS submissions
  FROM student_commits
  GROUP BY exercise_id, allowed_file_type_id
),
offered AS (
  SELECT e.id AS exercise_id, eaft.allowed_file_type_id AS file_type_id
  FROM scoped_exercises e
  JOIN exercises_allowed_file_types eaft ON eaft.exercise_id = e.id
),
pick AS (
  SELECT o.file_type_id,
         count(*) AS exercises_with_submissions,
         avg(coalesce(pet.submissions, 0)::float8 / et.submissions::float8) AS pick_rate
  FROM offered o
  JOIN exercise_totals et ON et.exercise_id = o.exercise_id
  LEFT JOIN per_exercise_type pet
    ON pet.exercise_id = o.exercise_id
   AND pet.file_type_id = o.file_type_id
  GROUP BY o.file_type_id
),
results AS (
  SELECT tc.id AS test_case_id,
         tc.exercise_id,
         c.allowed_file_type_id AS file_type_id,
         r.cpu_time::float8 AS cpu_time
  FROM commits_exercise_test_cases_results r
  JOIN commits c ON c.id = r.commit_id
  JOIN scoped_exercises e ON e.id = c.exercise_id
  JOIN exercises_test_cases tc ON tc.id = r.exercise_test_case_id
  WHERE c.allowed_file_type_id IS NOT NULL
    AND r.cpu_time >= 0
),
per_case AS (
  SELECT test_case_id,
         exercise_id,
         file_type_id,
         percentile_cont(0.5) WITHIN GROUP (ORDER BY cpu_time) AS median_cpu
  FROM results
  GROUP BY test_case_id, exercise_id, file_type_id
),
per_case_baseline AS (
  SELECT test_case_id,
         percentile_cont(0.5) WITHIN GROUP (ORDER BY median_cpu) AS baseline
  FROM per_case
  GROUP BY test_case_id
  HAVING count(*) >= $1
),
ratios AS (
  SELECT p.file_type_id,
         p.test_case_id,
         p.exercise_id,
         p.median_cpu,
         p.median_cpu / b.baseline AS ratio
  FROM per_case p
  JOIN per_case_baseline b ON b.test_case_id = p.test_case_id
  WHERE b.baseline > 0
),
perf AS (
  SELECT file_type_id,
         count(*) AS compared_test_cases,
         count(DISTINCT exercise_id) AS compared_exercises,
         percentile_cont(0.5) WITHIN GROUP (ORDER BY median_cpu) AS cpu_median_s,
         percentile_cont(0.5) WITHIN GROUP (ORDER BY ratio) AS relative_cpu,
         percentile_cont(0.25) WITHIN GROUP (ORDER BY ratio) AS relative_cpu_p25,
         percentile_cont(0.75) WITHIN GROUP (ORDER BY ratio) AS relative_cpu_p75
  FROM ratios
  GROUP BY file_type_id
),
all_results AS (
  SELECT file_type_id,
         count(*) AS results,
         count(DISTINCT test_case_id) AS test_cases
  FROM results
  GROUP BY file_type_id
)
SELECT cat.id, cat.name, cat.extension, cat.is_compilable, cat.is_available,
       coalesce(a.exercises_offered, 0),
       coalesce(a.offerings_offered, 0),
       coalesce(a.professors_offering, 0),
       coalesce(ch.submissions, 0),
       coalesce(ch.students, 0),
       coalesce(ch.exercises_chosen, 0),
       coalesce(pk.exercises_with_submissions, 0),
       pk.pick_rate,
       coalesce(ar.results, 0),
       coalesce(ar.test_cases, 0),
       coalesce(pf.compared_test_cases, 0),
       coalesce(pf.compared_exercises, 0),
       pf.cpu_median_s,
       pf.relative_cpu,
       pf.relative_cpu_p25,
       pf.relative_cpu_p75,
       t.exercises,
       t.professors,
       ct.submissions,
       ct.students,
       t.unclassified_submissions
FROM catalog cat
CROSS JOIN totals t
CROSS JOIN choice_totals ct
LEFT JOIN allowance a ON a.file_type_id = cat.id
LEFT JOIN choice ch ON ch.file_type_id = cat.id
LEFT JOIN pick pk ON pk.file_type_id = cat.id
LEFT JOIN perf pf ON pf.file_type_id = cat.id
LEFT JOIN all_results ar ON ar.file_type_id = cat.id
ORDER BY cat.name, cat.id
`

// percentOf returns part/total as a two-decimal percentage, or zero when the
// denominator is empty.
func percentOf(part, total int64) float64 {
	if total <= 0 {
		return 0
	}
	return roundTo(float64(part)/float64(total)*100, 2)
}

// percentOrNil converts a SQL fraction (0–1) to a percentage, preserving NULL.
func percentOrNil(value sql.NullFloat64) *float64 {
	if !value.Valid {
		return nil
	}
	percent := roundTo(value.Float64*100, 2)
	return &percent
}

// roundedOrNil rounds a SQL float, preserving NULL.
func roundedOrNil(value sql.NullFloat64, places int) *float64 {
	if !value.Valid {
		return nil
	}
	rounded := roundTo(value.Float64, places)
	return &rounded
}

// roundTo keeps accumulator noise out of the API and the table.
func roundTo(value float64, places int) float64 {
	scale := math.Pow(10, float64(places))
	return math.Round(value*scale) / scale
}
