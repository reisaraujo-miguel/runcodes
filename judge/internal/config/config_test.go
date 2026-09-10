package config

import (
	"testing"
	"time"
)

// clearEnv unsets every variable Load consults so defaults can be asserted.
func clearEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"JUDGE_ADDR", "JUDGE_AUTH_TOKEN", "RUNCODES_JUDGE_TOKEN",
		"JUDGE_CONCURRENCY", "JUDGE_POLL_INTERVAL", "JUDGE_EVENT_RETENTION",
		"JUDGE_PODMAN_URI", "CONTAINER_HOST", "JUDGE_IMAGE_FORMAT",
		"JUDGE_EXEC_DIR", "JUDGE_EXEC_DIR_REMOTE", "JUDGE_KEEP_WORKSPACES",
		"RUNCODES_DEFAULT_COMPILATION_TIMEOUT", "RUNCODES_DEFAULT_EXEC_TIMEOUT",
		"JUDGE_DEFAULT_CASE_TIMEOUT",
		"RUNCODES_DB_HOST", "RUNCODES_DB_PORT", "RUNCODES_DB_NAME",
		"RUNCODES_DB_USER", "RUNCODES_DB_USERNAME", "RUNCODES_DB_PASSWORD",
		"RUNCODES_DB_SSLMODE",
		"RUNCODES_S3_ENDPOINT", "RUNCODES_S3_REGION",
		"RUNCODES_S3_CREDENTIALS_KEY", "RUNCODES_S3_CREDENTIALS_SECRET",
		"RUNCODES_S3_BUCKET_PREFIX",
	} {
		t.Setenv(key, "")
	}
}

func TestLoadDefaults(t *testing.T) {
	clearEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Addr != ":9000" {
		t.Errorf("Addr = %q, want :9000", cfg.Addr)
	}
	if cfg.Concurrency != 4 {
		t.Errorf("Concurrency = %d, want 4", cfg.Concurrency)
	}
	if cfg.PollInterval != time.Second {
		t.Errorf("PollInterval = %s, want 1s", cfg.PollInterval)
	}
	if cfg.EventRetention != 10*time.Minute {
		t.Errorf("EventRetention = %s, want 10m", cfg.EventRetention)
	}
	if cfg.ExecDirRemote != cfg.ExecDir {
		t.Errorf("ExecDirRemote = %q, want ExecDir %q", cfg.ExecDirRemote, cfg.ExecDir)
	}
	if cfg.DB.User != "runcodes" {
		t.Errorf("DB.User = %q, want runcodes", cfg.DB.User)
	}
	// Derived pool sizing.
	if cfg.DB.MaxOpenConns != cfg.Concurrency+2 {
		t.Errorf("DB.MaxOpenConns = %d, want concurrency+2", cfg.DB.MaxOpenConns)
	}
	if cfg.DB.MaxIdleConns != cfg.Concurrency {
		t.Errorf("DB.MaxIdleConns = %d, want concurrency", cfg.DB.MaxIdleConns)
	}
}

func TestLoadOverrides(t *testing.T) {
	clearEnv(t)
	t.Setenv("JUDGE_CONCURRENCY", "8")
	t.Setenv("JUDGE_POLL_INTERVAL", "250ms")
	t.Setenv("JUDGE_AUTH_TOKEN", "s3cret")
	t.Setenv("RUNCODES_DB_USERNAME", "legacy")
	t.Setenv("RUNCODES_S3_BUCKET_PREFIX", "mycode")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Concurrency != 8 {
		t.Errorf("Concurrency = %d, want 8", cfg.Concurrency)
	}
	if cfg.PollInterval != 250*time.Millisecond {
		t.Errorf("PollInterval = %s, want 250ms", cfg.PollInterval)
	}
	if cfg.AuthToken != "s3cret" {
		t.Errorf("AuthToken = %q, want s3cret", cfg.AuthToken)
	}
	// RUNCODES_DB_USER falls back to RUNCODES_DB_USERNAME.
	if cfg.DB.User != "legacy" {
		t.Errorf("DB.User = %q, want legacy", cfg.DB.User)
	}
	if cfg.DB.MaxOpenConns != 10 { // 8 + 2
		t.Errorf("DB.MaxOpenConns = %d, want 10", cfg.DB.MaxOpenConns)
	}
	if cfg.S3.CommitsBucket() != "mycode-commits" {
		t.Errorf("CommitsBucket = %q", cfg.S3.CommitsBucket())
	}
}

func TestLoadRejectsInvalidConcurrency(t *testing.T) {
	clearEnv(t)
	t.Setenv("JUDGE_CONCURRENCY", "0")

	if _, err := Load(); err == nil {
		t.Fatal("expected an error for JUDGE_CONCURRENCY=0")
	}
}

func TestLoadBareSecondsDuration(t *testing.T) {
	clearEnv(t)
	t.Setenv("JUDGE_POLL_INTERVAL", "3")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.PollInterval != 3*time.Second {
		t.Errorf("PollInterval = %s, want 3s", cfg.PollInterval)
	}
}

func TestBucketNames(t *testing.T) {
	s := S3Config{BucketBase: "runcodes"}
	cases := map[string]string{
		s.CommitsBucket(): "runcodes-commits",
		s.CasesBucket():   "runcodes-cases",
		s.FilesBucket():   "runcodes-files",
		s.OutputsBucket(): "runcodes-outputfiles",
	}
	for got, want := range cases {
		if got != want {
			t.Errorf("bucket = %q, want %q", got, want)
		}
	}
}
