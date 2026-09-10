// Package config loads the judge's configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// Config holds every tunable of the judge service.
type Config struct {
	// HTTP API.
	Addr      string
	AuthToken string

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

	CompilationTimeout time.Duration
	BaseExecTimeout    time.Duration
	DefaultCaseTimeout time.Duration

	MonitorMaxFileSize int64
	MonitorMaxMemSize  int64
	MaxOutputFileSize  int64

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
		Addr:           env("JUDGE_ADDR", ":9000"),
		AuthToken:      env("JUDGE_AUTH_TOKEN", env("RUNCODES_JUDGE_TOKEN", "")),
		Concurrency:    envInt("JUDGE_CONCURRENCY", 4),
		PollInterval:   envDuration("JUDGE_POLL_INTERVAL", time.Second),
		EventRetention: envDuration("JUDGE_EVENT_RETENTION", 10*time.Minute),

		PodmanURI:   podmanURI(),
		ImageFormat: env("JUDGE_IMAGE_FORMAT", "ghcr.io/runcodes-icmc/compiler-images-%s:latest"),

		ExecDir:        env("JUDGE_EXEC_DIR", filepath.Join(os.TempDir(), "runcodes-judge")),
		KeepWorkspaces: envBool("JUDGE_KEEP_WORKSPACES", false),

		CompilationTimeout: envDuration("RUNCODES_DEFAULT_COMPILATION_TIMEOUT", 10*time.Second),
		BaseExecTimeout:    envDuration("RUNCODES_DEFAULT_EXEC_TIMEOUT", 5*time.Second),
		DefaultCaseTimeout: envDuration("JUDGE_DEFAULT_CASE_TIMEOUT", 3*time.Second),

		MonitorMaxFileSize: envInt64("JUDGE_MONITOR_MAX_FILE_SIZE", 5*1024*1024),
		MonitorMaxMemSize:  envInt64("JUDGE_MONITOR_MAX_MEM_SIZE", 256*1024*1024),
		MaxOutputFileSize:  envInt64("JUDGE_MAX_OUTPUT_FILE_SIZE", 1024*1024),

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
	}
	if c.PollInterval <= 0 {
		return fmt.Errorf("JUDGE_POLL_INTERVAL must be positive, got %s", c.PollInterval)
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
	}
	if uri := os.Getenv("CONTAINER_HOST"); uri != "" {
		return uri
	}
	return fmt.Sprintf("unix:///run/user/%d/podman/podman.sock", os.Getuid())
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envInt64(key string, def int64) int64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return def
}

func envBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

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
