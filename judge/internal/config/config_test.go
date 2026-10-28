package config

import (
	"fmt"
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
		"JUDGE_IMAGE_REGISTRY", "JUDGE_RUNNER_IMAGE_PREFIX", "JUDGE_RUNNER_IMAGE_TAG",
		"JUDGE_EXEC_DIR", "JUDGE_EXEC_DIR_REMOTE", "JUDGE_KEEP_WORKSPACES",
		"JUDGE_DEFAULT_COMPILATION_TIMEOUT", "JUDGE_COMPILATION_WAIT",
		"JUDGE_DEFAULT_EXEC_TIMEOUT", "JUDGE_DEFAULT_CASE_TIMEOUT",
		"JUDGE_MONITOR_MAX_FILE_SIZE", "JUDGE_MONITOR_MAX_MEM_SIZE",
		"JUDGE_CONTAINER_MEMORY_BYTES", "JUDGE_CONTAINER_PIDS_LIMIT",
		"JUDGE_CONTAINER_CPU_QUOTA", "JUDGE_CONTAINER_TMPFS_BYTES", "JUDGE_MAX_RUN_DURATION", "JUDGE_ARTIFACT_TTL",
		"JUDGE_MAX_WORKSPACE_BYTES", "JUDGE_MAX_WORKSPACE_ENTRIES",
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
	if cfg.ImageFormat != "ghcr.io/runcodes-icmc/runcodes-runner-%s:latest" {
		t.Errorf("ImageFormat = %q, want the default runner template", cfg.ImageFormat)
	}
	// The compilation limit is the image's; the judge only bounds its own patience.
	if cfg.CompilationTimeout != 0 {
		t.Errorf("CompilationTimeout = %s, want 0 (use the image's own)", cfg.CompilationTimeout)
	}
	if cfg.CompilationWait != 2*time.Minute {
		t.Errorf("CompilationWait = %s, want 2m", cfg.CompilationWait)
	}
	if cfg.MonitorMaxMemSize != 0 || cfg.MonitorMaxFileSize != 0 {
		t.Errorf("monitor limits = %d/%d, want 0/0 (use the image's own)",
			cfg.MonitorMaxMemSize, cfg.MonitorMaxFileSize)
	}
	if cfg.ArtifactTTL != 7*24*time.Hour {
		t.Errorf("ArtifactTTL = %s, want 168h", cfg.ArtifactTTL)
	}
	if cfg.ContainerTmpfsBytes != 256*1024*1024 {
		t.Errorf("ContainerTmpfsBytes = %d, want 256 MiB", cfg.ContainerTmpfsBytes)
	}
	if cfg.MaxWorkspaceBytes != 1024*1024*1024 {
		t.Errorf("MaxWorkspaceBytes = %d, want 1 GiB", cfg.MaxWorkspaceBytes)
	}
	if cfg.MaxWorkspaceEntries != 50_000 {
		t.Errorf("MaxWorkspaceEntries = %d, want 50000", cfg.MaxWorkspaceEntries)
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
	t.Setenv("JUDGE_ARTIFACT_TTL", "24h")
	t.Setenv("JUDGE_CONTAINER_TMPFS_BYTES", "67108864")
	t.Setenv("JUDGE_MAX_WORKSPACE_BYTES", "268435456")
	t.Setenv("JUDGE_MAX_WORKSPACE_ENTRIES", "1234")
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
	if cfg.ArtifactTTL != 24*time.Hour {
		t.Errorf("ArtifactTTL = %s, want 24h", cfg.ArtifactTTL)
	}
	if cfg.ContainerTmpfsBytes != 64*1024*1024 {
		t.Errorf("ContainerTmpfsBytes = %d, want 64 MiB", cfg.ContainerTmpfsBytes)
	}
	if cfg.MaxWorkspaceBytes != 256*1024*1024 {
		t.Errorf("MaxWorkspaceBytes = %d, want 256 MiB", cfg.MaxWorkspaceBytes)
	}
	if cfg.MaxWorkspaceEntries != 1234 {
		t.Errorf("MaxWorkspaceEntries = %d, want 1234", cfg.MaxWorkspaceEntries)
	}
}

func TestLoadRejectsNegativeArtifactTTL(t *testing.T) {
	clearEnv(t)
	t.Setenv("JUDGE_ARTIFACT_TTL", "-1h")

	if _, err := Load(); err == nil {
		t.Fatal("expected an error for a negative JUDGE_ARTIFACT_TTL")
	}
}

func TestLoadRejectsNonPositiveTmpfs(t *testing.T) {
	clearEnv(t)
	t.Setenv("JUDGE_CONTAINER_TMPFS_BYTES", "0")

	if _, err := Load(); err == nil {
		t.Fatal("expected an error for JUDGE_CONTAINER_TMPFS_BYTES=0")
	}
}

func TestLoadRejectsInvalidConcurrency(t *testing.T) {
	clearEnv(t)
	t.Setenv("JUDGE_CONCURRENCY", "0")

	if _, err := Load(); err == nil {
		t.Fatal("expected an error for JUDGE_CONCURRENCY=0")
	}
}

func TestRunnerImageFormat(t *testing.T) {
	tests := []struct {
		name       string
		registry   string
		prefix     string
		tag        string
		format     string
		wantFormat string
		wantImage  string
	}{
		{
			name:       "default",
			wantFormat: "ghcr.io/runcodes-icmc/runcodes-runner-%s:latest",
			wantImage:  "ghcr.io/runcodes-icmc/runcodes-runner-c:latest",
		},
		{
			name:       "custom registry, prefix and tag",
			registry:   "ghcr.io",
			prefix:     "reisaraujo-miguel/runcodes-runner",
			tag:        "v1.2.3",
			wantFormat: "ghcr.io/reisaraujo-miguel/runcodes-runner-%s:v1.2.3",
			wantImage:  "ghcr.io/reisaraujo-miguel/runcodes-runner-c:v1.2.3",
		},
		{
			name:       "trailing separators are ignored",
			registry:   "ghcr.io/",
			prefix:     "/reisaraujo-miguel/runcodes-runner-/",
			wantFormat: "ghcr.io/reisaraujo-miguel/runcodes-runner-%s:latest",
			wantImage:  "ghcr.io/reisaraujo-miguel/runcodes-runner-c:latest",
		},
		{
			name:       "blank tag falls back to latest",
			registry:   "ghcr.io",
			prefix:     "reisaraujo-miguel/runcodes-runner",
			tag:        "  ",
			wantFormat: "ghcr.io/reisaraujo-miguel/runcodes-runner-%s:latest",
			wantImage:  "ghcr.io/reisaraujo-miguel/runcodes-runner-c:latest",
		},
		{
			name:       "explicit format wins",
			registry:   "ghcr.io",
			prefix:     "reisaraujo-miguel/runcodes-runner",
			tag:        "v1.2.3",
			format:     "registry.example.com/team/runners/%s:edge",
			wantFormat: "registry.example.com/team/runners/%s:edge",
			wantImage:  "registry.example.com/team/runners/c:edge",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearEnv(t)
			if tt.registry != "" {
				t.Setenv("JUDGE_IMAGE_REGISTRY", tt.registry)
			}
			if tt.prefix != "" {
				t.Setenv("JUDGE_RUNNER_IMAGE_PREFIX", tt.prefix)
			}
			if tt.tag != "" {
				t.Setenv("JUDGE_RUNNER_IMAGE_TAG", tt.tag)
			}
			if tt.format != "" {
				t.Setenv("JUDGE_IMAGE_FORMAT", tt.format)
			}

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if cfg.ImageFormat != tt.wantFormat {
				t.Errorf("ImageFormat = %q, want %q", cfg.ImageFormat, tt.wantFormat)
			}
			if got := fmt.Sprintf(cfg.ImageFormat, "c"); got != tt.wantImage {
				t.Errorf("expanded image = %q, want %q", got, tt.wantImage)
			}
		})
	}
}

// The judge abandoning a run the container is still allowed to compile is the
// failure the two compilation knobs exist to prevent, so the pairing is checked
// rather than left to whoever sets both.
func TestLoadRejectsCompilationWaitShorterThanTheLimit(t *testing.T) {
	clearEnv(t)
	t.Setenv("JUDGE_COMPILATION_WAIT", "60s")
	t.Setenv("JUDGE_DEFAULT_COMPILATION_TIMEOUT", "60s")

	if _, err := Load(); err == nil {
		t.Fatal("expected an error when the wait equals the container's limit")
	}

	t.Setenv("JUDGE_COMPILATION_WAIT", "61s")
	if _, err := Load(); err != nil {
		t.Fatalf("Load with a longer wait: %v", err)
	}

	// With no override the wait stands alone: the image's limit is not known here.
	t.Setenv("JUDGE_DEFAULT_COMPILATION_TIMEOUT", "0")
	t.Setenv("JUDGE_COMPILATION_WAIT", "30s")
	if _, err := Load(); err != nil {
		t.Fatalf("Load without a compilation limit: %v", err)
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

func TestLoadRejectsNegativeWorkspaceLimits(t *testing.T) {
	clearEnv(t)
	t.Setenv("JUDGE_MAX_WORKSPACE_BYTES", "-1")

	if _, err := Load(); err == nil {
		t.Fatal("Load accepted a negative JUDGE_MAX_WORKSPACE_BYTES")
	}
}
