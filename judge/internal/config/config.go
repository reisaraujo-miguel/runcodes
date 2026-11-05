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

const (
	// DefaultRunnerImageRegistry is the registry that holds the language
	// images. The judge combines it with DefaultRunnerImagePrefix and
	// DefaultRunnerImageTag, so the default produces
	// "ghcr.io/runcodes-icmc/runcodes-runner-c:latest".
	DefaultRunnerImageRegistry = "ghcr.io"
	// DefaultRunnerImagePrefix is the image path inside the registry, without
	// the trailing hyphen or language suffix. The judge appends "-<language>"
	// and then the tag.
	DefaultRunnerImagePrefix = "runcodes-icmc/runcodes-runner"
	// DefaultRunnerImageTag is the tag pulled when the deployment does not pin
	// a release.
	DefaultRunnerImageTag = "latest"
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

	// CompilationTimeout is the compilation limit the container is told to
	// enforce. The runner images no longer carry a per-language default, so this
	// is the only compilation budget the harness gets and it must be positive.
	CompilationTimeout time.Duration
	// CompilationWait is how long the judge waits for the compilation milestones.
	// It is the judge's patience, not the container's limit: it must outlast
	// CompilationTimeout plus container startup, or the judge gives up on a
	// compilation the container is still allowed to finish.
	CompilationWait    time.Duration
	BaseExecTimeout    time.Duration
	DefaultCaseTimeout time.Duration

	// StartupWait bounds how long the judge waits for a container's *first*
	// milestone (`compilation.start`, or `run.start` for an interpreted
	// language): the container booting, the harness starting and the milestone
	// travelling back on the log stream.
	//
	// It is deliberately far above the normal cost (a few hundred milliseconds)
	// and far below a user's patience: a container that has not reached its first
	// milestone in this long is wedged, and making the student wait for the
	// compilation or execution budget would only delay the error. When it fires,
	// the run ends as server_error and an `alert` event tells the administrators.
	StartupWait time.Duration

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
	// ContainerTmpfsBytes caps the writable tmpfs at /tmp when the root
	// filesystem is read-only (compilers and runtimes need scratch space).
	ContainerTmpfsBytes int64

	// MaxCompareFileBytes bounds the two files a test case is graded from, so a
	// submission cannot force the judge to allocate without limit.
	MaxCompareFileBytes int64

	// MaxWorkspaceBytes and MaxWorkspaceEntries bound the aggregate workspace
	// (including files the submission creates) and are enforced while the
	// container runs. 0 disables a limit; the defaults are deliberately above
	// normal build output but below the point where one run can fill the host.
	MaxWorkspaceBytes   int64
	MaxWorkspaceEntries int64

	// MaxArtifactBytes bounds one generated file, not a run's total: each case
	// is normally bounded by its own file-size limit, and this value applies when
	// the case has none. It keeps a submission from publishing an unbounded file
	// without letting an earlier case's large file suppress a later case's.
	MaxArtifactBytes int64
	// MaxCommitArtifactBytes bounds everything one commit may publish. A suite
	// multiplies the per-case limit by its number of cases, so without this
	// aggregate cap a single submission can still store an unbounded amount in
	// object storage (and make the backend's zip endpoint stream it).
	MaxCommitArtifactBytes int64

	// MaxExtractFileBytes bounds a single entry, MaxExtractBytes the total
	// expanded size, and MaxExtractEntries the number of entries when unpacking
	// a zip submission. The byte limits alone do not stop an archive made of
	// millions of empty files from exhausting inodes.
	MaxExtractFileBytes int64
	MaxExtractBytes     int64
	MaxExtractEntries   int

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
		ImageFormat: runnerImageFormat(),

		ExecDir:        env("JUDGE_EXEC_DIR", filepath.Join(os.TempDir(), "runcodes-judge")),
		KeepWorkspaces: envBool("JUDGE_KEEP_WORKSPACES", false),

		// The harness always runs the compiler under this budget, so the default
		// matches the platform's documented 60s compilation limit. A deployment
		// that needs longer raises it together with JUDGE_COMPILATION_WAIT.
		CompilationTimeout: envDuration("JUDGE_DEFAULT_COMPILATION_TIMEOUT", 60*time.Second),
		CompilationWait:    envDuration("JUDGE_COMPILATION_WAIT", 2*time.Minute),
		// A container prints its first milestone within a second on a healthy
		// host. Thirty seconds is already a wedged run, and small enough that a
		// student sees the failure instead of a spinner.
		StartupWait:        envDuration("JUDGE_STARTUP_WAIT", 30*time.Second),
		BaseExecTimeout:    envDuration("JUDGE_DEFAULT_EXEC_TIMEOUT", 5*time.Second),
		DefaultCaseTimeout: envDuration("JUDGE_DEFAULT_CASE_TIMEOUT", 3*time.Second),
		MaxRunDuration:     envDuration("JUDGE_MAX_RUN_DURATION", 30*time.Minute),

		MonitorMaxFileSize: envInt64("JUDGE_MONITOR_MAX_FILE_SIZE", 256<<20),
		MonitorMaxMemSize:  envInt64("JUDGE_MONITOR_MAX_MEM_SIZE", 512<<20),
		MaxOutputFileSize:  envInt64("JUDGE_MAX_OUTPUT_FILE_SIZE", 1024*1024),

		// Above the largest per-language default a language image sets (1GB), so
		// that a run which exhausts its memory is reported as a memory limit by
		// the in-container monitor rather than killed by the cgroup as a signal.
		// Concurrency multiplies this: 4 workers at this value need 6GB of RAM.
		ContainerMemoryBytes: envInt64("JUDGE_CONTAINER_MEMORY_BYTES", 1536*1024*1024),
		ContainerPidsLimit:   envInt64("JUDGE_CONTAINER_PIDS_LIMIT", 256),
		ContainerCPUQuota:    envInt64("JUDGE_CONTAINER_CPU_QUOTA", 100000),
		ContainerTmpfsBytes:  envInt64("JUDGE_CONTAINER_TMPFS_BYTES", 256*1024*1024),

		MaxCompareFileBytes:    envInt64("JUDGE_MAX_COMPARE_FILE_BYTES", 16*1024*1024),
		MaxWorkspaceBytes:      envInt64("JUDGE_MAX_WORKSPACE_BYTES", 1024*1024*1024),
		MaxWorkspaceEntries:    envInt64("JUDGE_MAX_WORKSPACE_ENTRIES", 50_000),
		MaxArtifactBytes:       envInt64("JUDGE_MAX_ARTIFACT_BYTES", 64*1024*1024),
		MaxCommitArtifactBytes: envInt64("JUDGE_MAX_COMMIT_ARTIFACT_BYTES", 256*1024*1024),

		MaxExtractFileBytes: envInt64("JUDGE_MAX_EXTRACT_FILE_BYTES", 64*1024*1024),
		MaxExtractBytes:     envInt64("JUDGE_MAX_EXTRACT_BYTES", 256*1024*1024),
		MaxExtractEntries:   envInt("JUDGE_MAX_EXTRACT_ENTRIES", 1000),

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
	if c.MaxExtractEntries <= 0 {
		return fmt.Errorf("JUDGE_MAX_EXTRACT_ENTRIES must be positive, got %d", c.MaxExtractEntries)
	}

	// Container limits are optional (0 disables one), but a negative value is a
	// configuration mistake rather than a way to disable it.
	if c.ContainerMemoryBytes < 0 || c.ContainerPidsLimit < 0 || c.ContainerCPUQuota < 0 {
		return fmt.Errorf("container limits must not be negative")
	}

	if c.ContainerTmpfsBytes <= 0 {
		return fmt.Errorf("JUDGE_CONTAINER_TMPFS_BYTES must be positive")
	}

	// 0 means "use whatever the language image sets"; a negative value is a mistake.
	if c.MonitorMaxFileSize < 0 || c.MonitorMaxMemSize < 0 {
		return fmt.Errorf("monitor limits must not be negative")
	}

	// The harness requires a positive value: `timeout 0` disables the harness
	// bound entirely, which leaves a wedged compiler to the judge's wait and turns
	// a compilation timeout into a milestone timeout.
	if c.CompilationTimeout <= 0 {
		return fmt.Errorf("JUDGE_DEFAULT_COMPILATION_TIMEOUT must be positive")
	}

	if c.CompilationWait <= 0 {
		return fmt.Errorf("JUDGE_COMPILATION_WAIT must be positive, got %s", c.CompilationWait)
	}

	if c.StartupWait <= 0 {
		return fmt.Errorf("JUDGE_STARTUP_WAIT must be positive, got %s", c.StartupWait)
	}

	// The startup budget ends before the compilation budget begins: a container
	// that has not printed `compilation.start` cannot be compiling yet, so waiting
	// longer for the harness to start than for the whole compilation means one of
	// the two numbers is wrong.
	if c.StartupWait > c.CompilationWait {
		return fmt.Errorf(
			"JUDGE_STARTUP_WAIT (%s) must not exceed JUDGE_COMPILATION_WAIT (%s)",
			c.StartupWait, c.CompilationWait,
		)
	}

	// A judge that waits less than the container may compile gives up first, and the
	// run fails with a milestone timeout instead of reporting the compilation error
	// the student needs to see. Equal values race, so the wait has to be longer.
	if c.CompilationWait <= c.CompilationTimeout {
		return fmt.Errorf(
			"JUDGE_COMPILATION_WAIT (%s) must exceed JUDGE_DEFAULT_COMPILATION_TIMEOUT (%s)",
			c.CompilationWait, c.CompilationTimeout,
		)
	}

	if c.MaxCompareFileBytes <= 0 {
		return fmt.Errorf("JUDGE_MAX_COMPARE_FILE_BYTES must be positive")
	}

	// 0 disables a workspace limit; a negative value is a configuration mistake.
	if c.MaxWorkspaceBytes < 0 || c.MaxWorkspaceEntries < 0 {
		return fmt.Errorf("workspace limits must not be negative")
	}

	if c.MaxArtifactBytes <= 0 {
		return fmt.Errorf("JUDGE_MAX_ARTIFACT_BYTES must be positive")
	}

	if c.MaxCommitArtifactBytes <= 0 {
		return fmt.Errorf("JUDGE_MAX_COMMIT_ARTIFACT_BYTES must be positive")
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

/*
runnerImageFormat resolves the printf template used to name a runner image.

JUDGE_IMAGE_FORMAT is the full override and wins when set. Otherwise the
template is built from JUDGE_IMAGE_REGISTRY, JUDGE_RUNNER_IMAGE_PREFIX and
JUDGE_RUNNER_IMAGE_TAG: the judge joins registry and prefix with a slash,
appends the language suffix and the tag, so the default is
"ghcr.io/runcodes-icmc/runcodes-runner-%s:latest". A trailing slash on the
registry and leading/trailing slashes or a trailing hyphen on the prefix are
ignored, which lets the same prefix work in Compose and Ansible.
*/
func runnerImageFormat() string {
	if format := strings.TrimSpace(os.Getenv("JUDGE_IMAGE_FORMAT")); format != "" {
		return format
	}

	registry := strings.TrimRight(
		strings.TrimSpace(env("JUDGE_IMAGE_REGISTRY", DefaultRunnerImageRegistry)),
		"/",
	)
	if registry == "" {
		registry = DefaultRunnerImageRegistry
	}

	prefix := strings.Trim(
		strings.TrimSpace(env("JUDGE_RUNNER_IMAGE_PREFIX", DefaultRunnerImagePrefix)),
		"/-",
	)
	if prefix == "" {
		prefix = DefaultRunnerImagePrefix
	}

	tag := strings.TrimSpace(env("JUDGE_RUNNER_IMAGE_TAG", DefaultRunnerImageTag))
	if tag == "" {
		tag = DefaultRunnerImageTag
	}

	return registry + "/" + prefix + "-%s:" + tag
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
