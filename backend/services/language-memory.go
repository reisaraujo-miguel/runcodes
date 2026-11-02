package services

import (
	"context"
	"database/sql"
	"log/slog"
	"math"

	"github.com/runcodes-icmc/runcodes/database"
	"github.com/runcodes-icmc/runcodes/models"
)

/*
languageMemoryQuery computes the paired peak-memory comparison. It mirrors the
CPU index: per test case, each language is reduced to its median peak; the case's
baseline is the median across the languages that ran it; the language index is
the median of those ratios. Only known memory values (>= 0) enter. Coverage is
computed over every executed case result in the scope, so a report with many
legacy `-1` rows says so instead of silently comparing a biased subset.
*/
const languageMemoryQuery = `
WITH scoped_exercises AS (
  SELECT id
  FROM exercises
  WHERE NOT removed
    AND ($2::int IS NULL OR offering_id = $2)
),
commits_scope AS (
  SELECT c.id, c.allowed_file_type_id AS file_type_id
  FROM commits c
  JOIN scoped_exercises e ON e.id = c.exercise_id
  WHERE c.allowed_file_type_id IS NOT NULL
),
all_results AS (
  SELECT cs.file_type_id, count(*) AS total_results
  FROM commits_exercise_test_cases_results r
  JOIN commits_scope cs ON cs.id = r.commit_id
  GROUP BY cs.file_type_id
),
results AS (
  SELECT tc.id AS test_case_id,
         tc.exercise_id,
         c.allowed_file_type_id AS file_type_id,
         r.mem_usage::float8 AS mem
  FROM commits_exercise_test_cases_results r
  JOIN commits c ON c.id = r.commit_id
  JOIN scoped_exercises e ON e.id = c.exercise_id
  JOIN exercises_test_cases tc ON tc.id = r.exercise_test_case_id
  WHERE c.allowed_file_type_id IS NOT NULL
    AND r.mem_usage >= 0
),
known AS (
  SELECT file_type_id, count(*) AS known_results
  FROM results
  GROUP BY file_type_id
),
per_case AS (
  SELECT test_case_id,
         exercise_id,
         file_type_id,
         percentile_cont(0.5) WITHIN GROUP (ORDER BY mem) AS median_mem
  FROM results
  GROUP BY test_case_id, exercise_id, file_type_id
),
per_case_baseline AS (
  SELECT test_case_id,
         percentile_cont(0.5) WITHIN GROUP (ORDER BY median_mem) AS baseline
  FROM per_case
  GROUP BY test_case_id
  HAVING count(*) >= $1
),
ratios AS (
  SELECT p.file_type_id,
         p.test_case_id,
         p.exercise_id,
         p.median_mem,
         p.median_mem / b.baseline AS ratio
  FROM per_case p
  JOIN per_case_baseline b ON b.test_case_id = p.test_case_id
  WHERE b.baseline > 0
),
perf AS (
  SELECT file_type_id,
         count(*) AS compared_test_cases,
         count(DISTINCT exercise_id) AS compared_exercises,
         percentile_cont(0.5) WITHIN GROUP (ORDER BY ratio) AS relative,
         percentile_cont(0.25) WITHIN GROUP (ORDER BY ratio) AS p25,
         percentile_cont(0.75) WITHIN GROUP (ORDER BY ratio) AS p75
  FROM ratios
  GROUP BY file_type_id
),
raw AS (
  SELECT file_type_id,
         percentile_cont(0.5) WITHIN GROUP (ORDER BY median_mem) AS median_bytes
  FROM per_case
  GROUP BY file_type_id
)
SELECT k.file_type_id,
       coalesce(p.compared_test_cases, 0),
       coalesce(p.compared_exercises, 0),
       r.median_bytes, p.relative, p.p25, p.p75,
       k.known_results, a.total_results
FROM known k
JOIN all_results a ON a.file_type_id = k.file_type_id
JOIN raw r ON r.file_type_id = k.file_type_id
LEFT JOIN perf p ON p.file_type_id = k.file_type_id
ORDER BY k.file_type_id
`

/*
loadLanguageMemory reads the memory comparison by file type id. A nil offering
id means the platform; a non-nil id scopes every denominator to that class.
*/
func loadLanguageMemory(
	ctx context.Context, offeringID *int64,
) (map[int64]*models.MemoryStat, error) {
	var scope any
	if offeringID != nil {
		scope = *offeringID
	}

	rows, err := database.DB.QueryContext(ctx, languageMemoryQuery,
		minComparisonLanguages, scope,
	)
	if err != nil {
		slog.ErrorContext(ctx, "error loading language memory",
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	defer rows.Close()

	memory := make(map[int64]*models.MemoryStat)
	for rows.Next() {
		var (
			fileTypeID        int64
			comparedCases     int64
			comparedExercises int64
			knownResults      int64
			totalResults      int64
			medianBytes       sql.NullFloat64
			relative          sql.NullFloat64
			p25               sql.NullFloat64
			p75               sql.NullFloat64
		)

		if err := rows.Scan(
			&fileTypeID, &comparedCases, &comparedExercises,
			&medianBytes, &relative, &p25, &p75,
			&knownResults, &totalResults,
		); err != nil {
			slog.ErrorContext(ctx, "error scanning language memory",
				slog.String("error", err.Error()),
			)
			return nil, ErrServer
		}

		stat := &models.MemoryStat{
			ComparedTestCases: comparedCases,
			ComparedExercises: comparedExercises,
			KnownResults:      knownResults,
			TotalResults:      totalResults,
			Coverage:          percentOf(knownResults, totalResults),
		}
		stat.Relative = roundedOrNil(relative, 3)
		stat.P25 = roundedOrNil(p25, 3)
		stat.P75 = roundedOrNil(p75, 3)
		if medianBytes.Valid {
			value := int64(math.Round(medianBytes.Float64))
			stat.MedianBytes = &value
		}

		memory[fileTypeID] = stat
	}
	if err := rows.Err(); err != nil {
		slog.ErrorContext(ctx, "error iterating language memory",
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}

	return memory, nil
}
