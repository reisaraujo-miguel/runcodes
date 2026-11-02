package services

import (
	"context"
	"database/sql"
	"log/slog"

	"github.com/runcodes-icmc/runcodes/database"
	"github.com/runcodes-icmc/runcodes/models"
)

/*
languageTimingsQuery aggregates the phase rows for the platform or one class.

`commits_scope` is the denominator for coverage: every classified commit in the
scope. `phases` carries the rows, `agg` reduces each (language, phase) to a
median and quartiles, and the image median excludes cold pulls. A cold pull is
operational, not a property of the language image's warm startup.
*/
const languageTimingsQuery = `
WITH scoped_exercises AS (
  SELECT id
  FROM exercises
  WHERE NOT removed
    AND ($1::int IS NULL OR offering_id = $1)
),
commits_scope AS (
  SELECT c.id, c.allowed_file_type_id AS file_type_id
  FROM commits c
  JOIN scoped_exercises e ON e.id = c.exercise_id
  WHERE c.allowed_file_type_id IS NOT NULL
),
totals AS (
  SELECT file_type_id, count(*) AS commits
  FROM commits_scope
  GROUP BY file_type_id
),
phases AS (
  SELECT cs.file_type_id,
         t.phase::text AS phase,
         t.duration_ms,
         t.cpu_ms,
         t.mem_peak_bytes,
         coalesce((t.detail->>'pulled')::boolean, false) AS pulled
  FROM commit_phase_timings t
  JOIN commits_scope cs ON cs.id = t.commit_id
),
agg AS (
  SELECT file_type_id,
         phase,
         count(*) AS samples,
         count(*) FILTER (WHERE phase = 'image_ensure' AND pulled) AS cold_samples,
         count(*) FILTER (WHERE phase = 'image_ensure' AND NOT pulled) AS warm_samples,
         percentile_cont(0.5) WITHIN GROUP (ORDER BY duration_ms)
           FILTER (WHERE phase <> 'image_ensure' OR NOT pulled) AS median_ms,
         percentile_cont(0.25) WITHIN GROUP (ORDER BY duration_ms)
           FILTER (WHERE phase <> 'image_ensure' OR NOT pulled) AS p25_ms,
         percentile_cont(0.75) WITHIN GROUP (ORDER BY duration_ms)
           FILTER (WHERE phase <> 'image_ensure' OR NOT pulled) AS p75_ms,
         percentile_cont(0.5) WITHIN GROUP (ORDER BY cpu_ms)
           FILTER (WHERE cpu_ms IS NOT NULL) AS median_cpu_ms,
         percentile_cont(0.5) WITHIN GROUP (ORDER BY mem_peak_bytes)
           FILTER (WHERE mem_peak_bytes IS NOT NULL) AS median_mem_bytes
  FROM phases
  GROUP BY file_type_id, phase
)
SELECT a.file_type_id, a.phase, a.samples, a.cold_samples, a.warm_samples,
       a.median_ms, a.p25_ms, a.p75_ms, a.median_cpu_ms, a.median_mem_bytes,
       t.commits
FROM agg a
JOIN totals t ON t.file_type_id = a.file_type_id
ORDER BY a.file_type_id, a.phase
`

/*
loadLanguageTimings reads the phase medians and merges them into the report by
file type id. The caller owns the scope; a nil offering id means the platform.
*/
func loadLanguageTimings(
	ctx context.Context, offeringID *int64,
) (map[int64]models.LanguageTimings, error) {
	var scope any
	if offeringID != nil {
		scope = *offeringID
	}

	rows, err := database.DB.QueryContext(ctx, languageTimingsQuery, scope)
	if err != nil {
		slog.ErrorContext(ctx, "error loading language timings",
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	defer rows.Close()

	timings := make(map[int64]models.LanguageTimings)
	for rows.Next() {
		var (
			fileTypeID  int64
			phase       string
			samples     int64
			coldSamples int64
			warmSamples int64
			commits     int64
			medianMS    sql.NullFloat64
			p25MS       sql.NullFloat64
			p75MS       sql.NullFloat64
			cpuMS       sql.NullFloat64
			memBytes    sql.NullFloat64
		)

		if err := rows.Scan(
			&fileTypeID, &phase, &samples, &coldSamples, &warmSamples,
			&medianMS, &p25MS, &p75MS, &cpuMS, &memBytes, &commits,
		); err != nil {
			slog.ErrorContext(ctx, "error scanning language timings",
				slog.String("error", err.Error()),
			)
			return nil, ErrServer
		}

		stat := &models.PhaseStat{
			Samples:     samples,
			WarmSamples: warmSamples,
			ColdSamples: coldSamples,
			Coverage:    percentOf(samples, commits),
		}
		stat.MedianSeconds = millisecondsOrNil(medianMS)
		stat.P25Seconds = millisecondsOrNil(p25MS)
		stat.P75Seconds = millisecondsOrNil(p75MS)
		stat.CPUTimeMedianSeconds = millisecondsOrNil(cpuMS)
		if memBytes.Valid {
			value := int64(memBytes.Float64)
			stat.MemPeakBytes = &value
		}

		entry := timings[fileTypeID]
		switch phase {
		case "queue":
			entry.Queue = stat
		case "workspace_prepare":
			entry.Workspace = stat
		case "image_ensure":
			entry.Image = stat
		case "startup":
			entry.Startup = stat
		case "compile":
			entry.Compile = stat
		case "run":
			entry.Run = stat
		case "grade":
			entry.Grade = stat
		case "artifact":
			entry.Artifact = stat
		default:
			// Workspace sub-steps are persisted for operations but are not part
			// of the product breakdown yet.
			continue
		}
		timings[fileTypeID] = entry
	}
	if err := rows.Err(); err != nil {
		slog.ErrorContext(ctx, "error iterating language timings",
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}

	return timings, nil
}

// millisecondsOrNil converts a SQL millisecond value to rounded seconds,
// preserving NULL. Durations below one microsecond round to zero rather than
// disappearing: the sample count still says the phase ran.
func millisecondsOrNil(value sql.NullFloat64) *float64 {
	if !value.Valid {
		return nil
	}
	seconds := roundTo(value.Float64/1000, 6)
	return &seconds
}
