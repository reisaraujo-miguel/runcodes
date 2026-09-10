// Package store is the judge's PostgreSQL access layer. It owns the durable
// queue: workers claim `queued` commits with SELECT ... FOR UPDATE SKIP LOCKED,
// writing only the claim (status -> compiling). Everything else the judge needs
// (test cases, files, S3 keys) is read here too.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/lib/pq"

	"github.com/runcodes-icmc/judge/internal/config"
	"github.com/runcodes-icmc/judge/internal/model"
)

// Store wraps the database handle.
type Store struct {
	db *sql.DB
}

// New opens the connection pool. Connections are established lazily, so a
// database that is not up yet does not prevent the process from starting;
// readiness (and each claim) reports the failure instead.
func New(cfg *config.Config) (*Store, error) {
	db, err := sql.Open("postgres", cfg.DB.DSN())
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	db.SetMaxOpenConns(cfg.DB.MaxOpenConns)
	db.SetMaxIdleConns(cfg.DB.MaxIdleConns)
	return &Store{db: db}, nil
}

// Close releases the pool.
func (s *Store) Close() error { return s.db.Close() }

// Ping reports whether Postgres is reachable (used by /readyz).
func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

// Claim atomically takes the oldest `queued` commit, marking it `compiling`.
// It returns (nil, nil) when the queue is empty.
func (s *Store) Claim(ctx context.Context) (*model.Commit, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin claim tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var (
		c       model.Commit
		userID  sql.NullInt64
		s3Key   sql.NullString
		created sql.NullTime
	)
	err = tx.QueryRowContext(ctx, `
		SELECT id, user_id, exercise_id, created_at, s3_key
		FROM commits
		WHERE status = 'queued'
		ORDER BY created_at ASC
		FOR UPDATE SKIP LOCKED
		LIMIT 1`).Scan(&c.ID, &userID, &c.ExerciseID, &created, &s3Key)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("select queued commit: %w", err)
	}
	if !s3Key.Valid || s3Key.String == "" {
		return nil, fmt.Errorf("commit %d has no s3_key", c.ID)
	}
	c.S3Key = s3Key.String
	if userID.Valid {
		c.UserID = userID.Int64
	}
	if created.Valid {
		c.CreatedAt = created.Time
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE commits SET status = 'compiling', compilation_started = now()
		WHERE id = $1`, c.ID); err != nil {
		return nil, fmt.Errorf("claim commit %d: %w", c.ID, err)
	}

	// `ghost` exercises re-use the real exercise's test cases/files.
	err = tx.QueryRowContext(ctx, `
		SELECT CASE WHEN ghost AND real_id IS NOT NULL THEN real_id ELSE id END
		FROM exercises WHERE id = $1`, c.ExerciseID).Scan(&c.RealExerciseID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("exercise %d not found", c.ExerciseID)
	}
	if err != nil {
		return nil, fmt.Errorf("resolve real exercise: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit claim: %w", err)
	}
	return &c, nil
}

// FetchTestCases returns the test cases of an exercise (ordered by id), each
// with the paths of its attached files.
func (s *Store) FetchTestCases(ctx context.Context, exerciseID int64) ([]model.TestCase, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, exercise_id, input_type::text, expected_output_type::text,
		       show_input, show_expected_output, show_user_output,
		       cpu_time_limit_seconds, mem_usage_limit_bytes,
		       stack_limit_bytes, file_size_limit_bytes
		FROM exercises_test_cases
		WHERE exercise_id = $1
		ORDER BY id`, exerciseID)
	if err != nil {
		return nil, fmt.Errorf("query test cases: %w", err)
	}
	defer rows.Close()

	var cases []model.TestCase
	var ids []int64
	for rows.Next() {
		var tc model.TestCase
		if err := rows.Scan(
			&tc.ID, &tc.ExerciseID, &tc.InputType, &tc.ExpectedOutputType,
			&tc.ShowInput, &tc.ShowExpectedOutput, &tc.ShowUserOutput,
			&tc.CPUTimeLimit, &tc.MemUsageLimit, &tc.StackLimit, &tc.FileSizeLimit,
		); err != nil {
			return nil, fmt.Errorf("scan test case: %w", err)
		}
		cases = append(cases, tc)
		ids = append(ids, tc.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate test cases: %w", err)
	}
	if len(cases) == 0 {
		return cases, nil
	}

	frows, err := s.db.QueryContext(ctx, `
		SELECT exercise_test_case_id, path
		FROM exercises_test_cases_files
		WHERE exercise_test_case_id = ANY($1)`, pq.Array(ids))
	if err != nil {
		return nil, fmt.Errorf("query test case files: %w", err)
	}
	defer frows.Close()

	byID := make(map[int64][]string)
	for frows.Next() {
		var id int64
		var path string
		if err := frows.Scan(&id, &path); err != nil {
			return nil, fmt.Errorf("scan test case file: %w", err)
		}
		byID[id] = append(byID[id], path)
	}
	if err := frows.Err(); err != nil {
		return nil, fmt.Errorf("iterate test case files: %w", err)
	}
	for i := range cases {
		cases[i].Files = byID[cases[i].ID]
	}
	return cases, nil
}

// FetchCompilationFiles returns the exercise's extra source files (S3 paths).
func (s *Store) FetchCompilationFiles(ctx context.Context, exerciseID int64) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT path FROM exercises_compilation_files
		WHERE exercise_id = $1
		ORDER BY id`, exerciseID)
	if err != nil {
		return nil, fmt.Errorf("query compilation files: %w", err)
	}
	defer rows.Close()

	var paths []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, fmt.Errorf("scan compilation file: %w", err)
		}
		paths = append(paths, p)
	}
	return paths, rows.Err()
}
