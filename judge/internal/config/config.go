// Package config loads the judge's configuration from environment variables.
package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Config holds every tunable of the judge service.
type Config struct {
	// HTTP API.
	Addr      string
	AuthToken string
	// AllowInsecureAPI serves /v1 without a token. It exists for a local instance
	// that nobody else can reach; the default is to refuse those requests.
	AllowInsecureAPI bool

	// Worker pool.
	Concurrency    int
	PollInterval   time.Duration
	EventRetention time.Duration

	// Container execution.
	PodmanURI   string
	ImageFormat string
	ExecDir     string
	// ExecDirRemote is ExecDir as seen by the podman service. They only differ
	// when the judge runs in a container while podman runs on the host.
	ExecDirRemote string

	// CompilationTimeout is the compilation limit the container is told to enforce,
	// in seconds. Zero (the default) writes nothing and leaves each language image's
	// own value in place; a positive value overrides every language.
	CompilationTimeout time.Duration
	// CompilationWait is how long the judge waits for the compilation milestones.
	// It is the judge's patience, not the container's limit: it must outlast the
	// compilation timeout the image runs under (the images' largest is 60s) plus
	// container startup, or the judge gives up on a compilation the container is
	// still allowed to finish.
	CompilationWait    time.Duration
	BaseExecTimeout    time.Duration
	DefaultCaseTimeout time.Duration

	// MaxRunDuration bounds one claimed commit end to end, so a stalled image
	// pull, container create or S3 read cannot pin a worker slot forever. It must
	// stay above the longest legitimate run (the phase timeouts below) and be
	// aligned with the backend's RUNCODES_JUDGE_STALE_TIMEOUT.
	MaxRunDuration time.Duration

	MonitorMaxFileSize int64
	MonitorMaxMemSize  int64
	MaxOutputFileSize  int64

	// Container limits are the judge-side cgroup caps applied to every graded
	// container. They backstop the in-container monitor, which the submission
	// shares privileges with and can therefore defeat.
	ContainerMemoryBytes int64
	ContainerPidsLimit   int64
	ContainerCPUQuota    int64

	// MaxCompareFileBytes bounds the two files a test case is graded from, so a
	// submission cannot force the judge to allocate without limit.
	MaxCompareFileBytes int64

	// MaxArtifactBytes bounds the output archive published for a commit, so a
	// submission cannot fill the shared execution directory with output files.
	MaxArtifactBytes int64

	// MaxExtractFileBytes bounds a single entry and MaxExtractBytes the total
	// expanded size when unpacking a zip submission, so a zip bomb cannot fill
	// the shared execution directory.
	MaxExtractFileBytes int64
	MaxExtractBytes     int64

	KeepWorkspaces bool

	DB DBConfig
	S3 S3Config
}

// DBConfig describes the PostgreSQL connection.
type DBConfig struct {
	Host     string
	Port     string
	Name     string
	User     string
	Password string
	SSLMode  string

	MaxOpenConns int
	MaxIdleConns int
}

// DSN returns the lib/pq connection string.
func (d DBConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Password, d.Name, d.SSLMode,
	)
}

/*
LogInsecureTransportWarnings names what to set when the judge talks to Postgres
and S3 in clear text. Both defaults point at the compose network, where that is
intended; this is the reminder for a deployment that moves off it. It is separate
from validate so the messages go through the configured logger.
*/
func (c *Config) LogInsecureTransportWarnings() {
	if c.DB.SSLMode == "disable" {
		slog.Warn("database connections are not encrypted", "variable", "RUNCODES_DB_SSLMODE")
	}

	if strings.HasPrefix(c.S3.Endpoint, "http://") {
		slog.Warn("S3 connections are not encrypted",
			"endpoint", c.S3.Endpoint,
			"variable", "RUNCODES_S3_ENDPOINT",
		)
	}
}

// S3Config describes the SeaweedFS (S3-compatible) endpoint.
type S3Config struct {
	Endpoint   string
	Region     string
	AccessKey  string
	SecretKey  string
	BucketBase string
}

// Bucket returns "<prefix>-<suffix>", the platform's bucket naming scheme.
func (s S3Config) Bucket(suffix string) string { return s.BucketBase + "-" + suffix }

// CommitsBucket holds the submitted source files.
func (s S3Config) CommitsBucket() string { return s.Bucket("commits") }

// CasesBucket holds test-case inputs, expected outputs and extra files.
func (s S3Config) CasesBucket() string { return s.Bucket("cases") }

// FilesBucket holds exercise compilation files.
func (s S3Config) FilesBucket() string { return s.Bucket("files") }

// OutputsBucket holds generated output archives (stored by the backend).
func (s S3Config) OutputsBucket() string { return s.Bucket("outputfiles") }

// Load builds a Config from the environment, applying defaults.
func Load() (*Config, error) {
	cfg := &Config{
		Addr:             env("JUDGE_ADDR", ":9000"),
		AuthToken:        env("JUDGE_AUTH_TOKEN", env("RUNCODES_JUDGE_TOKEN", "")),
		AllowInsecureAPI: envBool("JUDGE_ALLOW_INSECURE", false),
		Concurrency:      envInt("JUDGE_CONCURRENCY", 4),
		PollInterval:     envDuration("JUDGE_POLL_INTERVAL", time.Second),
		EventRetention:   envDuration("JUDGE_EVENT_RETENTION", 10*time.Minute),

		PodmanURI:   podmanURI(),
		ImageFormat: env("JUDGE_IMAGE_FORMAT", "ghcr.io/runcodes-icmc/runcodes-runner-%s:latest"),

		ExecDir:        env("JUDGE_EXEC_DIR", filepath.Join(os.TempDir(), "runcodes-judge")),
		KeepWorkspaces: envBool("JUDGE_KEEP_WORKSPACES", false),

		// 0 leaves each language image's own compilation timeout in place: 10s for most
		// languages, 60s for Go, C# and Julia, 20s for Zig. A language knows how long
		// its compiler needs better than a single deployment-wide number does.
		CompilationTimeout: envDuration("JUDGE_DEFAULT_COMPILATION_TIMEOUT", 0),
		CompilationWait:    envDuration("JUDGE_COMPILATION_WAIT", 2*time.Minute),
		BaseExecTimeout:    envDuration("JUDGE_DEFAULT_EXEC_TIMEOUT", 5*time.Second),
		DefaultCaseTimeout: envDuration("JUDGE_DEFAULT_CASE_TIMEOUT", 3*time.Second),
		MaxRunDuration:     envDuration("JUDGE_MAX_RUN_DURATION", 30*time.Minute),

		MonitorMaxFileSize: envInt64("JUDGE_MONITOR_MAX_FILE_SIZE", 0),
		MonitorMaxMemSize:  envInt64("JUDGE_MONITOR_MAX_MEM_SIZE", 0),
		MaxOutputFileSize:  envInt64("JUDGE_MAX_OUTPUT_FILE_SIZE", 1024*1024),

		// Above the largest per-language default a language image sets (1GB), so
		// that a run which exhausts its memory is reported as a memory limit by
		// the in-container monitor rather than killed by the cgroup as a signal.
		// Concurrency multiplies this: 4 workers at this value need 6GB of RAM.
		ContainerMemoryBytes: envInt64("JUDGE_CONTAINER_MEMORY_BYTES", 1536*1024*1024),
		ContainerPidsLimit:   envInt64("JUDGE_CONTAINER_PIDS_LIMIT", 256),
		ContainerCPUQuota:    envInt64("JUDGE_CONTAINER_CPU_QUOTA", 100000),

		MaxCompareFileBytes: envInt64("JUDGE_MAX_COMPARE_FILE_BYTES", 16*1024*1024),
		MaxArtifactBytes:    envInt64("JUDGE_MAX_ARTIFACT_BYTES", 64*1024*1024),

		MaxExtractFileBytes: envInt64("JUDGE_MAX_EXTRACT_FILE_BYTES", 64*1024*1024),
		MaxExtractBytes:     envInt64("JUDGE_MAX_EXTRACT_BYTES", 256*1024*1024),

		DB: DBConfig{
			Host:     env("RUNCODES_DB_HOST", "localhost"),
			Port:     env("RUNCODES_DB_PORT", "5432"),
			Name:     env("RUNCODES_DB_NAME", "runcodes"),
			User:     env("RUNCODES_DB_USER", env("RUNCODES_DB_USERNAME", "runcodes")),
			Password: env("RUNCODES_DB_PASSWORD", ""),
			SSLMode:  env("RUNCODES_DB_SSLMODE", "disable"),
		},
		S3: S3Config{
			Endpoint:   env("RUNCODES_S3_ENDPOINT", "http://localhost:8333"),
			Region:     env("RUNCODES_S3_REGION", "sa-east-1"),
			AccessKey:  env("RUNCODES_S3_CREDENTIALS_KEY", "test_key"),
			SecretKey:  env("RUNCODES_S3_CREDENTIALS_SECRET", "test_secret"),
			BucketBase: env("RUNCODES_S3_BUCKET_PREFIX", "runcodes"),
		},
	}

	if cfg.ExecDirRemote = os.Getenv("JUDGE_EXEC_DIR_REMOTE"); cfg.ExecDirRemote == "" {
		cfg.ExecDirRemote = cfg.ExecDir
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) validate() error {
	if c.Concurrency < 1 {
		return fmt.Errorf("JUDGE_CONCURRENCY must be >= 1, got %d", c.Concurrency)
	} else if c.PollInterval <= 0 {
		return fmt.Errorf("JUDGE_POLL_INTERVAL must be positive, got %s", c.PollInterval)
	}

	if c.MaxExtractFileBytes <= 0 || c.MaxExtractBytes <= 0 {
		return fmt.Errorf("extraction size limits must be positive")
	} else if c.MaxExtractFileBytes > c.MaxExtractBytes {
		return fmt.Errorf("JUDGE_MAX_EXTRACT_FILE_BYTES must not exceed JUDGE_MAX_EXTRACT_BYTES")
	}

	// Container limits are optional (0 disables one), but a negative value is a
	// configuration mistake rather than a way to disable it.
	if c.ContainerMemoryBytes < 0 || c.ContainerPidsLimit < 0 || c.ContainerCPUQuota < 0 {
		return fmt.Errorf("container limits must not be negative")
	}

	// 0 means "use whatever the language image sets"; a negative value is a mistake.
	if c.MonitorMaxFileSize < 0 || c.MonitorMaxMemSize < 0 {
		return fmt.Errorf("monitor limits must not be negative")
	}

	// Bounded because the judge also waits this long for the compilation milestones.
	if c.CompilationTimeout < 0 {
		return fmt.Errorf("JUDGE_DEFAULT_COMPILATION_TIMEOUT must not be negative")
	}

	if c.CompilationWait <= 0 {
		return fmt.Errorf("JUDGE_COMPILATION_WAIT must be positive, got %s", c.CompilationWait)
	}

	// A judge that waits less than the container may compile gives up first, and the
	// run fails with a milestone timeout instead of reporting the compilation error
	// the student needs to see. Equal values race, so the wait has to be longer.
	if c.CompilationTimeout > 0 && c.CompilationWait <= c.CompilationTimeout {
		return fmt.Errorf(
			"JUDGE_COMPILATION_WAIT (%s) must exceed JUDGE_DEFAULT_COMPILATION_TIMEOUT (%s)",
			c.CompilationWait, c.CompilationTimeout,
		)
	}

	if c.MaxCompareFileBytes <= 0 {
		return fmt.Errorf("JUDGE_MAX_COMPARE_FILE_BYTES must be positive")
	}

	if c.MaxArtifactBytes <= 0 {
		return fmt.Errorf("JUDGE_MAX_ARTIFACT_BYTES must be positive")
	}

	if c.MaxRunDuration <= 0 {
		return fmt.Errorf("JUDGE_MAX_RUN_DURATION must be positive")
	}

	if c.DB.MaxIdleConns == 0 {
		c.DB.MaxIdleConns = c.Concurrency
	}

	if c.DB.MaxOpenConns == 0 {
		c.DB.MaxOpenConns = c.Concurrency + 2
	}

	return nil
}

func podmanURI() string {
	if uri := os.Getenv("JUDGE_PODMAN_URI"); uri != "" {
		return uri
	} else if uri := os.Getenv("CONTAINER_HOST"); uri != "" {
		return uri
	}

	return fmt.Sprintf("unix:///run/user/%d/podman/podman.sock", os.Getuid())
}

// env returns the value of the environment variable named by the key.
func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return def
}

// envInt returns the value of the environment variable named by the key, parsed as an int.
func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

// envInt64 returns the value of the environment variable named by the key, parsed as an int64.
func envInt64(key string, def int64) int64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return def
}

// envBool returns the value of the environment variable named by the key, parsed as a bool.
func envBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

// envDuration returns the value of the environment variable named by the key, parsed as a time.Duration.
func envDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		// Accept both a bare number of seconds and a Go duration ("10s").
		if n, err := strconv.Atoi(v); err == nil {
			return time.Duration(n) * time.Second
		}
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}
