//go:build integration

// Package main hosts the judge's end-to-end integration test. It runs only with
// the `integration` build tag and requires a real PostgreSQL, an S3-compatible
// store (SeaweedFS) and a rootless podman socket; see integration/run.sh.
//
// The test seeds a commit and a test case, uploads the source and the case I/O,
// claims the commit through the real store, and drives the real engine against a
// real language image, asserting the emitted event stream.
package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	_ "github.com/lib/pq"

	"github.com/runcodes-icmc/judge/internal/config"
	"github.com/runcodes-icmc/judge/internal/engine"
	"github.com/runcodes-icmc/judge/internal/events"
	"github.com/runcodes-icmc/judge/internal/podman"
	"github.com/runcodes-icmc/judge/internal/storage"
	"github.com/runcodes-icmc/judge/internal/store"
)

const cSource = `#include <stdio.h>
int main(void) {
    printf("hello\n");
    return 0;
}
`

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func s3ConfigFromEnv() config.S3Config {
	return config.S3Config{
		Endpoint:   envOr("RUNCODES_S3_ENDPOINT", "http://127.0.0.1:8333"),
		Region:     envOr("RUNCODES_S3_REGION", "sa-east-1"),
		AccessKey:  envOr("RUNCODES_S3_CREDENTIALS_KEY", "test_key"),
		SecretKey:  envOr("RUNCODES_S3_CREDENTIALS_SECRET", "test_secret"),
		BucketBase: envOr("RUNCODES_S3_BUCKET_PREFIX", "runcodes-itest"),
	}
}

func TestJudgeRunsLanguageImageEndToEnd(t *testing.T) {
	dsn := os.Getenv("JUDGE_TEST_DB_DSN")
	if dsn == "" {
		t.Skip("JUDGE_TEST_DB_DSN is not set; start the harness with integration/run.sh")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()
	st := store.NewWithDB(db)
	if err := st.Ping(ctx); err != nil {
		t.Skipf("database unreachable: %v", err)
	}

	podmanURI := envOr("JUDGE_PODMAN_URI", fmt.Sprintf("unix:///run/user/%d/podman/podman.sock", os.Getuid()))
	pc := podman.New(podmanURI)
	if err := pc.Ready(ctx); err != nil {
		t.Skipf("podman unreachable at %s: %v", podmanURI, err)
	}

	s3cfg := s3ConfigFromEnv()
	s3client := newS3Client(ctx, t, s3cfg)
	for _, bucket := range []string{s3cfg.CommitsBucket(), s3cfg.CasesBucket(), s3cfg.FilesBucket()} {
		ensureBucket(ctx, t, s3client, bucket)
	}

	// --- Seed an exercise, a test case and a queued commit --------------------
	var exerciseID, caseID, commitID int64
	if err := db.QueryRowContext(ctx, `
		INSERT INTO exercises (title, deadline, open_date)
		VALUES ($1, now() + interval '1 day', now() - interval '1 day')
		RETURNING id`, "judge integration test").Scan(&exerciseID); err != nil {
		t.Fatalf("insert exercise: %v", err)
	}
	if err := db.QueryRowContext(ctx, `
		INSERT INTO exercises_test_cases
		    (exercise_id, input, input_type, expected_output, expected_output_type,
		     cpu_time_limit_seconds, mem_usage_limit_bytes, stack_limit_bytes, file_size_limit_bytes)
		VALUES ($1, '', 'text', 'hello', 'text', 5, $2, 0, 0)
		RETURNING id`, exerciseID, 256*1024*1024).Scan(&caseID); err != nil {
		t.Fatalf("insert test case: %v", err)
	}

	sourceKey := fmt.Sprintf("itest-%d/main.c", time.Now().UnixNano())
	if err := db.QueryRowContext(ctx, `
		INSERT INTO commits (exercise_id, status, s3_key)
		VALUES ($1, 'queued', $2)
		RETURNING id`, exerciseID, sourceKey).Scan(&commitID); err != nil {
		t.Fatalf("insert commit: %v", err)
	}

	putObject(ctx, t, s3client, s3cfg.CommitsBucket(), sourceKey, []byte(cSource))
	putObject(ctx, t, s3client, s3cfg.CasesBucket(), fmt.Sprintf("%d/in", caseID), []byte(""))
	putObject(ctx, t, s3client, s3cfg.CasesBucket(), fmt.Sprintf("%d/out", caseID), []byte("hello\n"))

	t.Cleanup(func() {
		bg := context.Background()
		_, _ = db.ExecContext(bg, `DELETE FROM commits WHERE id = $1`, commitID)
		_, _ = db.ExecContext(bg, `DELETE FROM exercises_test_cases WHERE id = $1`, caseID)
		_, _ = db.ExecContext(bg, `DELETE FROM exercises WHERE id = $1`, exerciseID)
		for _, obj := range []struct{ bucket, key string }{
			{s3cfg.CommitsBucket(), sourceKey},
			{s3cfg.CasesBucket(), fmt.Sprintf("%d/in", caseID)},
			{s3cfg.CasesBucket(), fmt.Sprintf("%d/out", caseID)},
		} {
			_, _ = s3client.DeleteObject(bg, &s3.DeleteObjectInput{
				Bucket: aws.String(obj.bucket), Key: aws.String(obj.key),
			})
		}
	})

	// --- Build the real engine ------------------------------------------------
	execDir := t.TempDir()
	cfg := &config.Config{
		Concurrency:        1,
		ExecDir:            execDir,
		ExecDirRemote:      execDir,
		ImageFormat:        envOr("JUDGE_TEST_IMAGE_FORMAT", "ghcr.io/runcodes-icmc/compiler-images-%s:latest"),
		CompilationTimeout: 90 * time.Second,
		BaseExecTimeout:    15 * time.Second,
		DefaultCaseTimeout: 10 * time.Second,
		MonitorMaxFileSize: 5 * 1024 * 1024,
		MonitorMaxMemSize:  256 * 1024 * 1024,
		MaxOutputFileSize:  1024 * 1024,
		KeepWorkspaces:     os.Getenv("JUDGE_TEST_KEEP") == "true",
		S3:                 s3cfg,
	}

	s3store, err := storage.NewS3(ctx, cfg.S3)
	if err != nil {
		t.Fatalf("build s3 store: %v", err)
	}
	hub := events.New(2 * time.Minute)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	eng := engine.New(cfg, st, s3store, pc, hub, logger)

	// --- Claim and run --------------------------------------------------------
	commit, err := st.Claim(ctx)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if commit == nil {
		t.Fatal("no queued commit was claimed")
	}
	if commit.ID != commitID {
		t.Fatalf("claimed commit %d, want %d (is the database shared?)", commit.ID, commitID)
	}
	t.Logf("claimed commit %d (exercise %d, real exercise %d)", commit.ID, commit.ExerciseID, commit.RealExerciseID)

	sub := hub.Subscribe(commitID, 0)
	defer sub.Cancel()

	eng.Process(ctx, commit)

	// `finished` closes the subscription channel.
	frames := make(chan []events.Frame, 1)
	go func() {
		var collected []events.Frame
		for frame := range sub.Ch {
			collected = append(collected, frame)
		}
		frames <- collected
	}()

	var collected []events.Frame
	select {
	case collected = <-frames:
	case <-time.After(4 * time.Minute):
		t.Fatal("timed out waiting for the run to finish")
	}

	for _, frame := range collected {
		t.Logf("event %-12s seq=%d %s", frame.Name, frame.Seq, frame.Data)
	}

	// --- Assertions -----------------------------------------------------------
	var (
		sawCompiled   bool
		sawCaseResult bool
		sawFinished   bool
		finishStatus  string
		numCorrect    int
		score         float64
	)
	for _, frame := range collected {
		switch frame.Name {
		case events.NameCompilation:
			var p struct {
				Compiled bool `json:"compiled"`
			}
			_ = json.Unmarshal(frame.Data, &p)
			sawCompiled = sawCompiled || p.Compiled
		case events.NameCaseResult:
			var p struct {
				TestCaseID int64  `json:"test_case_id"`
				Status     string `json:"status"`
			}
			_ = json.Unmarshal(frame.Data, &p)
			if p.TestCaseID == caseID && p.Status == "correct" {
				sawCaseResult = true
			}
		case events.NameFinished:
			var p struct {
				Status          string  `json:"status"`
				NumCorrectCases int     `json:"num_correct_cases"`
				Score           float64 `json:"score"`
			}
			_ = json.Unmarshal(frame.Data, &p)
			sawFinished = true
			finishStatus = p.Status
			numCorrect = p.NumCorrectCases
			score = p.Score
		}
	}

	if !sawCompiled {
		t.Error("no successful compilation event")
	}
	if !sawCaseResult {
		t.Error("no correct case_result for the seeded test case")
	}
	if !sawFinished {
		t.Fatal("no finished event")
	}
	if finishStatus != "completed" {
		t.Errorf("finished status = %q, want completed", finishStatus)
	}
	if numCorrect != 1 {
		t.Errorf("num_correct_cases = %d, want 1", numCorrect)
	}
	if score != 100 {
		t.Errorf("score = %v, want 100", score)
	}

	// The output archive should have been produced.
	if info, err := os.Stat(eng.ArtifactPath(commitID)); err != nil {
		t.Errorf("output archive missing: %v", err)
	} else if info.Size() == 0 {
		t.Error("output archive is empty")
	}
}

func newS3Client(ctx context.Context, t *testing.T, cfg config.S3Config) *s3.Client {
	t.Helper()
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKey, cfg.SecretKey, "",
		)),
	)
	if err != nil {
		t.Fatalf("load aws config: %v", err)
	}
	return s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(cfg.Endpoint)
		o.UsePathStyle = true
	})
}

func ensureBucket(ctx context.Context, t *testing.T, client *s3.Client, name string) {
	t.Helper()
	if _, err := client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(name)}); err != nil {
		// An existing bucket makes CreateBucket fail; the upload below would
		// surface a genuinely missing bucket.
		t.Logf("CreateBucket(%s): %v", name, err)
	}
}

func putObject(ctx context.Context, t *testing.T, client *s3.Client, bucket, key string, body []byte) {
	t.Helper()
	if _, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(body),
	}); err != nil {
		t.Fatalf("put s3://%s/%s: %v", bucket, key, err)
	}
}
