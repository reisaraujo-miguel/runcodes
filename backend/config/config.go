/*
Package config loads the backend's configuration from environment variables.

Every value is read once, at startup, by Load; the resulting Config is published
as C and read from there for the rest of the process lifetime, so no other
package needs to know an environment variable's name, default or format. A
required value that is missing or a value that cannot be parsed is an error, so
a misconfigured deployment fails at startup instead of silently falling back to
a default.
*/
package config

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Defaults for the optional settings.
const (
	defaultFrontendOrigin = "http://localhost:5173"
	defaultRedisAddr      = "redis:6379"
	defaultS3Endpoint     = "http://seaweed:8333"
	defaultS3Region       = "sa-east-1"
	defaultS3Prefix       = "runcodes"
	defaultJudgeURL       = "http://judge:9000"
	defaultStaleTimeout   = 15 * time.Minute
)

// Config holds every tunable of the backend service.
type Config struct {
	// HTTP server.
	Addr           string // RUNCODES_API_PORT
	Debug          bool   // DEBUG_MODE
	FrontendOrigin string // FRONTEND_ORIGIN

	// Authentication.
	JWTSecret string // RUNCODES_JWT_SECRET
	// LegacyPasswordSalt is the old system's global salt. It is only needed to
	// verify - and then upgrade - the passwords of users migrated from it.
	LegacyPasswordSalt string // RUNCODES_LEGACY_PASSWORD_SALT

	DB    DBConfig
	Redis RedisConfig
	S3    S3Config
	Judge JudgeConfig
}

// DBConfig describes the PostgreSQL connection.
type DBConfig struct {
	Host     string
	Port     string
	Name     string
	User     string
	Password string
	SSLMode  string
}

// DSN returns the lib/pq connection string.
func (d DBConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Password, d.Name, d.SSLMode,
	)
}

// RedisConfig describes the cache. The cache is optional: an address that does
// not answer degrades the cache to a no-op instead of failing requests.
type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

// S3Config describes the SeaweedFS (S3-compatible) endpoint.
type S3Config struct {
	Endpoint     string
	Region       string
	AccessKey    string
	SecretKey    string
	BucketPrefix string
}

// Bucket returns "<prefix>-<suffix>", the platform's bucket naming scheme.
func (s S3Config) Bucket(suffix string) string { return s.BucketPrefix + "-" + suffix }

// CommitsBucket holds the submitted source files.
func (s S3Config) CommitsBucket() string { return s.Bucket("commits") }

// CasesBucket holds test-case inputs, expected outputs and extra files.
func (s S3Config) CasesBucket() string { return s.Bucket("cases") }

// FilesBucket holds exercise compilation files and attachments.
func (s S3Config) FilesBucket() string { return s.Bucket("files") }

// OutputFilesBucket holds generated output archives.
func (s S3Config) OutputFilesBucket() string { return s.Bucket("outputfiles") }

// JudgeConfig describes how to reach the judge and how long a claimed commit
// may stay in flight before the reconciliation sweeper gives up on it.
type JudgeConfig struct {
	URL          string
	Token        string
	StaleTimeout time.Duration
}

// C is the process configuration, published by Load. The rest of the process
// treats it as read-only; the single exception is main enabling debug mode from
// the -debug flag before any consumer runs.
var C *Config

// Get returns the loaded configuration. It panics when called before Load,
// which would be a startup wiring mistake rather than a runtime condition.
func Get() *Config {
	if C == nil {
		panic("config: Get called before Load")
	}
	return C
}

// Load builds the configuration from the environment, applying defaults, and
// publishes it as C.
func Load() (*Config, error) {
	debug, err := envBool("DEBUG_MODE", false)
	if err != nil {
		return nil, err
	}

	redisDB, err := envInt("RUNCODES_REDIS_DB", 0)
	if err != nil {
		return nil, err
	}

	staleTimeout, err := envDuration("RUNCODES_JUDGE_STALE_TIMEOUT", defaultStaleTimeout)
	if err != nil {
		return nil, err
	}

	// A prefix of only dashes trims down to nothing, which would leave the
	// buckets named "-commits" and so on.
	prefix := strings.Trim(env("RUNCODES_S3_BUCKET_PREFIX", defaultS3Prefix), "-")
	if prefix == "" {
		prefix = defaultS3Prefix
	}

	cfg := &Config{
		Addr:           os.Getenv("RUNCODES_API_PORT"),
		Debug:          debug,
		FrontendOrigin: env("FRONTEND_ORIGIN", defaultFrontendOrigin),

		JWTSecret:          os.Getenv("RUNCODES_JWT_SECRET"),
		LegacyPasswordSalt: os.Getenv("RUNCODES_LEGACY_PASSWORD_SALT"),

		DB: DBConfig{
			Host:     os.Getenv("RUNCODES_DB_HOST"),
			Port:     os.Getenv("RUNCODES_DB_PORT"),
			Name:     os.Getenv("RUNCODES_DB_NAME"),
			User:     os.Getenv("RUNCODES_DB_USER"),
			Password: os.Getenv("RUNCODES_DB_PASSWORD"),
			SSLMode:  env("RUNCODES_DB_SSLMODE", "disable"),
		},
		Redis: RedisConfig{
			Addr:     env("RUNCODES_REDIS_ADDR", defaultRedisAddr),
			Password: os.Getenv("RUNCODES_REDIS_PASSWORD"),
			DB:       redisDB,
		},
		S3: S3Config{
			Endpoint:     env("RUNCODES_S3_ENDPOINT", defaultS3Endpoint),
			Region:       env("RUNCODES_S3_REGION", defaultS3Region),
			AccessKey:    os.Getenv("RUNCODES_S3_CREDENTIALS_KEY"),
			SecretKey:    os.Getenv("RUNCODES_S3_CREDENTIALS_SECRET"),
			BucketPrefix: prefix,
		},
		Judge: JudgeConfig{
			// The base URL is concatenated with a path, so keep no trailing slash.
			URL:          strings.TrimRight(env("RUNCODES_JUDGE_URL", defaultJudgeURL), "/"),
			Token:        os.Getenv("RUNCODES_JUDGE_TOKEN"),
			StaleTimeout: staleTimeout,
		},
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	C = cfg

	return cfg, nil
}

func (c *Config) validate() error {
	required := []struct {
		name  string
		value string
	}{
		{"RUNCODES_API_PORT", c.Addr},
		{"RUNCODES_JWT_SECRET", c.JWTSecret},
		{"RUNCODES_DB_HOST", c.DB.Host},
		{"RUNCODES_DB_PORT", c.DB.Port},
		{"RUNCODES_DB_USER", c.DB.User},
		{"RUNCODES_DB_PASSWORD", c.DB.Password},
		{"RUNCODES_DB_NAME", c.DB.Name},
	}
	for _, field := range required {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("%s environment variable is not set", field.name)
		}
	}

	if err := checkPort("RUNCODES_API_PORT", c.Addr); err != nil {
		return err
	}
	if err := checkPort("RUNCODES_DB_PORT", c.DB.Port); err != nil {
		return err
	}

	// http.NewRequest fails on anything else, but only once a submission is
	// registered; catching it here points at the configuration instead.
	judgeURL, err := url.Parse(c.Judge.URL)
	if err != nil || (judgeURL.Scheme != "http" && judgeURL.Scheme != "https") ||
		judgeURL.Host == "" {
		return fmt.Errorf(
			"RUNCODES_JUDGE_URL must be an absolute http(s) URL, got %q", c.Judge.URL,
		)
	}

	if c.Judge.StaleTimeout <= 0 {
		return fmt.Errorf(
			"RUNCODES_JUDGE_STALE_TIMEOUT must be positive, got %s", c.Judge.StaleTimeout,
		)
	}

	if c.Redis.DB < 0 {
		return fmt.Errorf("RUNCODES_REDIS_DB must not be negative, got %d", c.Redis.DB)
	}

	// The frontend is served same-origin through the proxy, so CORS only matters
	// for a separate origin (a local `bun run dev`, say). The default is the
	// development origin, so a real cross-origin deployment that forgets to set
	// FRONTEND_ORIGIN is still rejected rather than silently allowed.
	if c.FrontendOrigin == "*" {
		return fmt.Errorf("FRONTEND_ORIGIN must be a single origin, not a wildcard")
	}
	if _, err := url.Parse(c.FrontendOrigin); err != nil {
		return fmt.Errorf("FRONTEND_ORIGIN must be a valid origin, got %q", c.FrontendOrigin)
	}

	return nil
}

/*
LogInsecureTransportWarnings names the variables to set when the process talks to
its dependencies in clear text. It is kept out of validate so the messages are
emitted through the configured logger, and so a plaintext internal network is not
mistaken for a startup error.
*/
func (c *Config) LogInsecureTransportWarnings() {
	if c.DB.SSLMode == "disable" {
		slog.Warn("database connections are not encrypted",
			"variable", "RUNCODES_DB_SSLMODE",
		)
	}
}

// checkPort validates that a value is a usable TCP port number.
func checkPort(name, value string) error {
	port, err := strconv.Atoi(value)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf(
			"%s must be a port number between 1 and 65535, got %q", name, value,
		)
	}

	return nil
}

// env returns the value of the environment variable named by the key, or def
// when it is unset or empty.
func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return def
}

// envInt returns the parsed int value of the environment variable named by the
// key, or def when it is unset or empty.
func envInt(key string, def int) (int, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return def, nil
	}

	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer, got %q", key, raw)
	}

	return n, nil
}

// envBool returns the parsed bool value of the environment variable named by
// the key, or def when it is unset or empty.
func envBool(key string, def bool) (bool, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return def, nil
	}

	b, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean, got %q", key, raw)
	}

	return b, nil
}

// envDuration returns the parsed duration value of the environment variable
// named by the key, or def when it is unset or empty. Only the Go duration
// syntax is accepted: a bare number has no obvious unit, and reading "15" as
// fifteen seconds where fifteen minutes were meant would silently break the
// feature it configures.
func envDuration(key string, def time.Duration) (time.Duration, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return def, nil
	}

	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf(
			"%s must be a duration such as 15m, got %q", key, raw,
		)
	}

	return d, nil
}
