package services

import (
	"context"
	"database/sql"
	"log/slog"

	"github.com/lib/pq"
	"github.com/runcodes-icmc/runcodes/config"
	"github.com/runcodes-icmc/runcodes/database"

	"github.com/runcodes-icmc/runcodes/models"
)

/*
Limits are resolved in three layers, from the most specific to the most general:

	the test case's override for the language the student submitted
	the exercise's default for that language
	the platform default

Each field resolves on its own, so a case may override the CPU time and inherit
the memory limit from the exercise. Sources record which layer won, which is what
the professor sees next to the effective value.
*/

// queryer is the subset of *sql.DB and *sql.Tx the limit queries use.
type queryer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// PlatformLimits is the configured default set, the last layer of the chain.
func PlatformLimits() models.PlatformLimits {
	limits := config.Get().Limits
	return models.PlatformLimits{
		Limits: models.Limits{
			CPUTimeLimitSeconds: limits.CPUTimeLimitSeconds,
			MemUsageLimitBytes:  limits.MemUsageLimitBytes,
			StackLimitBytes:     limits.StackLimitBytes,
			FileSizeLimitBytes:  limits.FileSizeLimitBytes,
		},
	}
}

// limitLayer ties one layer's overrides to the source it represents.
type limitLayer struct {
	overrides models.LimitOverrides
	source    models.LimitSource
}

/*
resolveLimits applies the layers to the platform default. The layers are given
from the most general to the most specific, so a later non-nil value wins and
carries its source.
*/
func resolveLimits(
	platform models.Limits, layers ...limitLayer,
) (models.Limits, models.LimitSources) {
	resolved := platform
	sources := models.LimitSources{
		CPUTimeLimitSeconds: models.LimitSourcePlatform,
		MemUsageLimitBytes:  models.LimitSourcePlatform,
		StackLimitBytes:     models.LimitSourcePlatform,
		FileSizeLimitBytes:  models.LimitSourcePlatform,
	}

	for _, layer := range layers {
		if layer.overrides.CPUTimeLimitSeconds != nil {
			resolved.CPUTimeLimitSeconds = *layer.overrides.CPUTimeLimitSeconds
			sources.CPUTimeLimitSeconds = layer.source
		}
		if layer.overrides.MemUsageLimitBytes != nil {
			resolved.MemUsageLimitBytes = *layer.overrides.MemUsageLimitBytes
			sources.MemUsageLimitBytes = layer.source
		}
		if layer.overrides.StackLimitBytes != nil {
			resolved.StackLimitBytes = *layer.overrides.StackLimitBytes
			sources.StackLimitBytes = layer.source
		}
		if layer.overrides.FileSizeLimitBytes != nil {
			resolved.FileSizeLimitBytes = *layer.overrides.FileSizeLimitBytes
			sources.FileSizeLimitBytes = layer.source
		}
	}

	return resolved, sources
}

// allowedLanguage is one language an exercise accepts.
type allowedLanguage struct {
	ID        int64
	Name      string
	Extension string
}

// loadAllowedLanguages returns the languages the professor allowed on an exercise.
func loadAllowedLanguages(
	ctx context.Context, q queryer, exerciseID int64,
) ([]allowedLanguage, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT aft.id, aft.name, aft.extension
		FROM exercises_allowed_file_types eaft
		JOIN allowed_file_types aft ON aft.id = eaft.allowed_file_type_id
		WHERE eaft.exercise_id = $1
		ORDER BY aft.name, aft.id`, exerciseID)
	if err != nil {
		slog.ErrorContext(ctx, "error loading the allowed languages",
			slog.Int64("exercise_id", exerciseID),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	defer rows.Close()

	languages := make([]allowedLanguage, 0, 4)
	for rows.Next() {
		var language allowedLanguage
		if err := rows.Scan(&language.ID, &language.Name, &language.Extension); err != nil {
			slog.ErrorContext(ctx, "error scanning an allowed language",
				slog.String("error", err.Error()),
			)
			return nil, ErrServer
		}
		languages = append(languages, language)
	}
	if err := rows.Err(); err != nil {
		return nil, ErrServer
	}
	return languages, nil
}

// loadExerciseOverrides reads the exercise's per-language defaults.
func loadExerciseOverrides(
	ctx context.Context, q queryer, exerciseID int64,
) (map[int64]models.LimitOverrides, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT allowed_file_type_id, cpu_time_limit_seconds,
		       mem_usage_limit_bytes, stack_limit_bytes, file_size_limit_bytes
		FROM exercise_language_limits
		WHERE exercise_id = $1`, exerciseID)
	if err != nil {
		slog.ErrorContext(ctx, "error loading the exercise language limits",
			slog.Int64("exercise_id", exerciseID),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	defer rows.Close()

	overrides := make(map[int64]models.LimitOverrides)
	for rows.Next() {
		var languageID int64
		var override models.LimitOverrides
		if err := rows.Scan(
			&languageID, &override.CPUTimeLimitSeconds,
			&override.MemUsageLimitBytes, &override.StackLimitBytes,
			&override.FileSizeLimitBytes,
		); err != nil {
			slog.ErrorContext(ctx, "error scanning an exercise language limit",
				slog.String("error", err.Error()),
			)
			return nil, ErrServer
		}
		overrides[languageID] = override
	}
	if err := rows.Err(); err != nil {
		return nil, ErrServer
	}
	return overrides, nil
}

// loadCaseOverrides reads the per-language overrides of the given test cases.
func loadCaseOverrides(
	ctx context.Context, q queryer, caseIDs []int64,
) (map[int64]map[int64]models.LimitOverrides, error) {
	overrides := make(map[int64]map[int64]models.LimitOverrides)
	if len(caseIDs) == 0 {
		return overrides, nil
	}

	rows, err := q.QueryContext(ctx, `
		SELECT exercise_test_case_id, allowed_file_type_id, cpu_time_limit_seconds,
		       mem_usage_limit_bytes, stack_limit_bytes, file_size_limit_bytes
		FROM test_case_language_limits
		WHERE exercise_test_case_id = ANY($1)`, pq.Array(caseIDs))
	if err != nil {
		slog.ErrorContext(ctx, "error loading the test case language limits",
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	defer rows.Close()

	for rows.Next() {
		var caseID, languageID int64
		var override models.LimitOverrides
		if err := rows.Scan(
			&caseID, &languageID, &override.CPUTimeLimitSeconds,
			&override.MemUsageLimitBytes, &override.StackLimitBytes,
			&override.FileSizeLimitBytes,
		); err != nil {
			slog.ErrorContext(ctx, "error scanning a test case language limit",
				slog.String("error", err.Error()),
			)
			return nil, ErrServer
		}
		if overrides[caseID] == nil {
			overrides[caseID] = make(map[int64]models.LimitOverrides)
		}
		overrides[caseID][languageID] = override
	}
	if err := rows.Err(); err != nil {
		return nil, ErrServer
	}
	return overrides, nil
}

/*
TestCaseLanguageLimits resolves the limits of each test case for every language
the exercise allows. The result is keyed by test case id; a case with no language
resolves to an empty slice.
*/
func TestCaseLanguageLimits(
	ctx context.Context, exerciseID int64, caseIDs []int64,
) (map[int64][]models.LanguageLimits, error) {
	languages, err := loadAllowedLanguages(ctx, database.DB, exerciseID)
	if err != nil {
		return nil, err
	}
	exerciseOverrides, err := loadExerciseOverrides(ctx, database.DB, exerciseID)
	if err != nil {
		return nil, err
	}
	caseOverrides, err := loadCaseOverrides(ctx, database.DB, caseIDs)
	if err != nil {
		return nil, err
	}

	platform := PlatformLimits().Limits
	resolved := make(map[int64][]models.LanguageLimits, len(caseIDs))
	for _, caseID := range caseIDs {
		perLanguage := make([]models.LanguageLimits, 0, len(languages))
		for _, language := range languages {
			exerciseLayer := limitLayer{
				overrides: exerciseOverrides[language.ID],
				source:    models.LimitSourceExercise,
			}
			caseLayer := limitLayer{
				overrides: caseOverrides[caseID][language.ID],
				source:    models.LimitSourceCase,
			}
			limits, sources := resolveLimits(platform, exerciseLayer, caseLayer)
			perLanguage = append(perLanguage, models.LanguageLimits{
				AllowedFileTypeID: language.ID,
				Name:              language.Name,
				Extension:         language.Extension,
				Overrides:         caseOverrides[caseID][language.ID],
				Limits:            limits,
				Sources:           sources,
			})
		}
		resolved[caseID] = perLanguage
	}
	return resolved, nil
}

/*
ListExerciseLanguageLimits returns the exercise's per-language defaults with the
values and sources that apply when a test case has no override of its own.
*/
func ListExerciseLanguageLimits(
	ctx context.Context, exerciseID int64, claims map[string]any,
) ([]models.LanguageLimits, error) {
	if _, err := requireExerciseAuthor(ctx, exerciseID, claims); err != nil {
		return nil, err
	}

	languages, err := loadAllowedLanguages(ctx, database.DB, exerciseID)
	if err != nil {
		return nil, err
	}
	overrides, err := loadExerciseOverrides(ctx, database.DB, exerciseID)
	if err != nil {
		return nil, err
	}

	platform := PlatformLimits().Limits
	limits := make([]models.LanguageLimits, 0, len(languages))
	for _, language := range languages {
		exerciseLayer := limitLayer{
			overrides: overrides[language.ID],
			source:    models.LimitSourceExercise,
		}
		resolved, sources := resolveLimits(platform, exerciseLayer)
		limits = append(limits, models.LanguageLimits{
			AllowedFileTypeID: language.ID,
			Name:              language.Name,
			Extension:         language.Extension,
			Overrides:         overrides[language.ID],
			Limits:            resolved,
			Sources:           sources,
		})
	}
	return limits, nil
}

/*
ReplaceExerciseLanguageLimits stores the exercise's per-language defaults. The
payload is the whole set: a language left out, or with every field nil, inherits
the platform default.
*/
func ReplaceExerciseLanguageLimits(
	ctx context.Context, exerciseID int64, in []models.LanguageLimits,
	claims map[string]any,
) ([]models.LanguageLimits, error) {
	acc, err := requireExerciseAuthor(ctx, exerciseID, claims)
	if err != nil {
		return nil, err
	}
	authoringID := acc.authoringExerciseID()

	languages, err := loadAllowedLanguages(ctx, database.DB, authoringID)
	if err != nil {
		return nil, err
	}
	if err := validateLanguageLimits(languages, in); err != nil {
		return nil, err
	}

	tx, err := database.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, ErrServer
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM exercise_language_limits WHERE exercise_id = $1`,
		authoringID,
	); err != nil {
		slog.ErrorContext(ctx, "error clearing the exercise language limits",
			slog.Int64("exercise_id", authoringID),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	if err := InsertExerciseLanguageLimits(ctx, tx, authoringID, in); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, ErrServer
	}

	return ListExerciseLanguageLimits(ctx, exerciseID, claims)
}

/*
InsertExerciseLanguageLimits writes an exercise's per-language defaults that
were validated by the caller. It is the shared writer for the exercise defaults
endpoint and the class archive import, which both know the exercise's languages
already.
*/
func InsertExerciseLanguageLimits(
	ctx context.Context, tx *sql.Tx, exerciseID int64, in []models.LanguageLimits,
) error {
	for _, language := range in {
		if language.Overrides.Empty() {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO exercise_language_limits
			    (exercise_id, allowed_file_type_id, cpu_time_limit_seconds,
			     mem_usage_limit_bytes, stack_limit_bytes, file_size_limit_bytes)
			VALUES ($1, $2, $3, $4, $5, $6)`,
			exerciseID, language.AllowedFileTypeID,
			language.Overrides.CPUTimeLimitSeconds,
			language.Overrides.MemUsageLimitBytes,
			language.Overrides.StackLimitBytes,
			language.Overrides.FileSizeLimitBytes,
		); err != nil {
			slog.ErrorContext(ctx, "error storing an exercise language limit",
				slog.Int64("exercise_id", exerciseID),
				slog.Int64("allowed_file_type_id", language.AllowedFileTypeID),
				slog.String("error", err.Error()),
			)
			return ErrServer
		}
	}
	return nil
}

/*
validateLanguageLimits checks that every entry names a language the exercise
allows and that the values are usable: a CPU time limit must be positive, and a
byte limit may be zero (no limit) but not negative.
*/
func validateLanguageLimits(
	languages []allowedLanguage, in []models.LanguageLimits,
) error {
	allowed := make(map[int64]struct{}, len(languages))
	for _, language := range languages {
		allowed[language.ID] = struct{}{}
	}

	seen := make(map[int64]struct{}, len(in))
	for _, language := range in {
		if _, ok := allowed[language.AllowedFileTypeID]; !ok {
			return ErrInvalidLimits
		}
		if _, duplicate := seen[language.AllowedFileTypeID]; duplicate {
			return ErrInvalidLimits
		}
		seen[language.AllowedFileTypeID] = struct{}{}

		overrides := language.Overrides
		if overrides.CPUTimeLimitSeconds != nil && *overrides.CPUTimeLimitSeconds <= 0 {
			return ErrInvalidLimits
		}
		if overrides.MemUsageLimitBytes != nil && *overrides.MemUsageLimitBytes < 0 {
			return ErrInvalidLimits
		}
		if overrides.StackLimitBytes != nil && *overrides.StackLimitBytes < 0 {
			return ErrInvalidLimits
		}
		if overrides.FileSizeLimitBytes != nil && *overrides.FileSizeLimitBytes < 0 {
			return ErrInvalidLimits
		}
	}
	return nil
}

/*
ReplaceTestCaseLanguageLimits stores a test case's per-language overrides inside
the caller's transaction. The payload is the whole set for the case.
*/
func ReplaceTestCaseLanguageLimits(
	ctx context.Context, tx *sql.Tx, caseID, exerciseID int64,
	in []models.LanguageLimits,
) error {
	languages, err := loadAllowedLanguages(ctx, tx, exerciseID)
	if err != nil {
		return err
	}
	if err := validateLanguageLimits(languages, in); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM test_case_language_limits WHERE exercise_test_case_id = $1`,
		caseID,
	); err != nil {
		slog.ErrorContext(ctx, "error clearing the test case language limits",
			slog.Int64("test_case_id", caseID),
			slog.String("error", err.Error()),
		)
		return ErrServer
	}
	return InsertTestCaseLanguageLimits(ctx, tx, caseID, in)
}

/*
InsertTestCaseLanguageLimits writes overrides that were validated by the caller.
It is the shared writer for the single-case API, the batch import and the class
archive import, which all know the exercise's languages already.
*/
func InsertTestCaseLanguageLimits(
	ctx context.Context, tx *sql.Tx, caseID int64, in []models.LanguageLimits,
) error {
	for _, language := range in {
		if language.Overrides.Empty() {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO test_case_language_limits
			    (exercise_test_case_id, allowed_file_type_id, cpu_time_limit_seconds,
			     mem_usage_limit_bytes, stack_limit_bytes, file_size_limit_bytes)
			VALUES ($1, $2, $3, $4, $5, $6)`,
			caseID, language.AllowedFileTypeID,
			language.Overrides.CPUTimeLimitSeconds,
			language.Overrides.MemUsageLimitBytes,
			language.Overrides.StackLimitBytes,
			language.Overrides.FileSizeLimitBytes,
		); err != nil {
			slog.ErrorContext(ctx, "error storing a test case language limit",
				slog.Int64("test_case_id", caseID),
				slog.String("error", err.Error()),
			)
			return ErrServer
		}
	}
	return nil
}

/*
SnapshotCommitLimits freezes the limits a submission runs under. Every value is
read from the commit row itself: the exercise it was submitted to (resolving a
ghost exercise to the one that owns the cases) and the language the backend
matched at submission time. A submission with no language match runs under the
platform defaults. Grading reads this snapshot, so editing the exercise after a
submission does not change it.

Every path that queues a commit must call this inside the same transaction, so a
commit the judge can claim never lacks a snapshot. The call is idempotent, which
lets a retry after a partial failure finish the rows it was missing.
*/
func SnapshotCommitLimits(
	ctx context.Context, tx *sql.Tx, commitIDs []int64,
) error {
	if len(commitIDs) == 0 {
		return nil
	}

	platform := PlatformLimits()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO commit_test_case_limits
		    (commit_id, exercise_test_case_id, cpu_time_limit_seconds,
		     mem_usage_limit_bytes, stack_limit_bytes, file_size_limit_bytes)
		SELECT c.id, tc.id,
		       COALESCE(tcl.cpu_time_limit_seconds, ell.cpu_time_limit_seconds, $1),
		       COALESCE(tcl.mem_usage_limit_bytes, ell.mem_usage_limit_bytes, $2),
		       COALESCE(tcl.stack_limit_bytes, ell.stack_limit_bytes, $3),
		       COALESCE(tcl.file_size_limit_bytes, ell.file_size_limit_bytes, $4)
		FROM commits c
		JOIN exercises e ON e.id = c.exercise_id
		JOIN exercises_test_cases tc
		  ON tc.exercise_id = CASE
		       WHEN e.ghost AND e.real_id IS NOT NULL THEN e.real_id
		       ELSE e.id
		     END
		LEFT JOIN test_case_language_limits tcl
		  ON tcl.exercise_test_case_id = tc.id
		 AND tcl.allowed_file_type_id = c.allowed_file_type_id
		LEFT JOIN exercise_language_limits ell
		  ON ell.exercise_id = tc.exercise_id
		 AND ell.allowed_file_type_id = c.allowed_file_type_id
		WHERE c.id = ANY($5)
		ON CONFLICT DO NOTHING`,
		platform.CPUTimeLimitSeconds, platform.MemUsageLimitBytes,
		platform.StackLimitBytes, platform.FileSizeLimitBytes,
		pq.Array(commitIDs),
	); err != nil {
		slog.ErrorContext(ctx, "error snapshotting the commit limits",
			slog.Any("commit_ids", commitIDs),
			slog.String("error", err.Error()),
		)
		return ErrServer
	}
	return nil
}
