package config

import (
	"strings"
	"testing"
	"time"
)

// clearEnv unsets every variable Load consults so defaults can be asserted.
func clearEnv(t *testing.T) {
	t.Helper()

	for _, key := range []string{
		"RUNCODES_API_PORT", "DEBUG_MODE", "FRONTEND_ORIGIN",
		"RUNCODES_JWT_SECRET", "RUNCODES_LEGACY_PASSWORD_SALT",
		"RUNCODES_DB_HOST", "RUNCODES_DB_PORT", "RUNCODES_DB_USER",
		"RUNCODES_DB_PASSWORD", "RUNCODES_DB_NAME", "RUNCODES_DB_SSLMODE",
		"RUNCODES_REDIS_ADDR", "RUNCODES_REDIS_PASSWORD", "RUNCODES_REDIS_DB",
		"RUNCODES_S3_ENDPOINT", "RUNCODES_S3_REGION",
		"RUNCODES_S3_CREDENTIALS_KEY", "RUNCODES_S3_CREDENTIALS_SECRET",
		"RUNCODES_S3_BUCKET_PREFIX",
		"RUNCODES_JUDGE_URL", "RUNCODES_JUDGE_TOKEN",
		"RUNCODES_JUDGE_STALE_TIMEOUT",
	} {
		t.Setenv(key, "")
	}
}

// setRequired sets the values Load insists on, so a test can then assert a
// specific default or a specific malformed value without triggering the
// required-field checks.
func setRequired(t *testing.T, overrides map[string]string) {
	t.Helper()

	required := map[string]string{
		"RUNCODES_API_PORT":    "8443",
		"RUNCODES_JWT_SECRET":  "secret",
		"RUNCODES_DB_HOST":     "database",
		"RUNCODES_DB_PORT":     "5432",
		"RUNCODES_DB_USER":     "runcodes",
		"RUNCODES_DB_PASSWORD": "password",
		"RUNCODES_DB_NAME":     "runcodes",
	}
	for key, value := range required {
		t.Setenv(key, value)
	}
	for key, value := range overrides {
		t.Setenv(key, value)
	}
}

func TestLoadDefaults(t *testing.T) {
	clearEnv(t)
	setRequired(t, nil)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Addr != "8443" {
		t.Errorf("Addr = %q, want 8443", cfg.Addr)
	}
	if cfg.Debug {
		t.Error("Debug = true, want false")
	}
	if cfg.FrontendOrigin != defaultFrontendOrigin {
		t.Errorf("FrontendOrigin = %q, want %q", cfg.FrontendOrigin, defaultFrontendOrigin)
	}
	if cfg.DB.SSLMode != "disable" {
		t.Errorf("DB.SSLMode = %q, want disable", cfg.DB.SSLMode)
	}
	if cfg.Redis.Addr != defaultRedisAddr || cfg.Redis.DB != 0 {
		t.Errorf("Redis = %+v, want the default address and db 0", cfg.Redis)
	}
	if cfg.S3.Endpoint != defaultS3Endpoint || cfg.S3.Region != defaultS3Region {
		t.Errorf("S3 = %+v, want the default endpoint and region", cfg.S3)
	}
	if cfg.S3.CommitsBucket() != defaultS3Prefix+"-commits" {
		t.Errorf("CommitsBucket = %q", cfg.S3.CommitsBucket())
	}
	if cfg.Judge.URL != defaultJudgeURL {
		t.Errorf("Judge.URL = %q, want %q", cfg.Judge.URL, defaultJudgeURL)
	}
	if cfg.Judge.StaleTimeout != defaultStaleTimeout {
		t.Errorf("Judge.StaleTimeout = %s, want %s", cfg.Judge.StaleTimeout, defaultStaleTimeout)
	}

	// Load publishes the configuration for the rest of the process.
	if C != cfg || Get() != cfg {
		t.Error("Load did not publish the configuration as C")
	}
}

func TestLoadReadsEveryValue(t *testing.T) {
	clearEnv(t)
	setRequired(t, map[string]string{
		"DEBUG_MODE":                     "true",
		"FRONTEND_ORIGIN":                "https://runcodes.example",
		"RUNCODES_LEGACY_PASSWORD_SALT":  "old-salt",
		"RUNCODES_DB_SSLMODE":            "require",
		"RUNCODES_REDIS_ADDR":            "cache:6379",
		"RUNCODES_REDIS_PASSWORD":        "redis-secret",
		"RUNCODES_REDIS_DB":              "3",
		"RUNCODES_S3_ENDPOINT":           "http://s3:8333",
		"RUNCODES_S3_REGION":             "us-east-1",
		"RUNCODES_S3_CREDENTIALS_KEY":    "key",
		"RUNCODES_S3_CREDENTIALS_SECRET": "secret",
		"RUNCODES_S3_BUCKET_PREFIX":      "platform",
		"RUNCODES_JUDGE_URL":             "http://judge:9000",
		"RUNCODES_JUDGE_TOKEN":           "token",
		"RUNCODES_JUDGE_STALE_TIMEOUT":   "3m",
	})

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if !cfg.Debug {
		t.Error("Debug = false, want true")
	}
	if cfg.FrontendOrigin != "https://runcodes.example" {
		t.Errorf("FrontendOrigin = %q", cfg.FrontendOrigin)
	}
	if cfg.LegacyPasswordSalt != "old-salt" {
		t.Errorf("LegacyPasswordSalt = %q", cfg.LegacyPasswordSalt)
	}
	if cfg.DB.SSLMode != "require" {
		t.Errorf("DB.SSLMode = %q", cfg.DB.SSLMode)
	}
	if cfg.Redis != (RedisConfig{Addr: "cache:6379", Password: "redis-secret", DB: 3}) {
		t.Errorf("Redis = %+v", cfg.Redis)
	}
	if cfg.S3.AccessKey != "key" || cfg.S3.SecretKey != "secret" {
		t.Errorf("S3 credentials = %q/%q", cfg.S3.AccessKey, cfg.S3.SecretKey)
	}
	if got := cfg.S3.OutputFilesBucket(); got != "platform-outputfiles" {
		t.Errorf("OutputFilesBucket = %q", got)
	}
	if cfg.Judge.Token != "token" || cfg.Judge.StaleTimeout != 3*time.Minute {
		t.Errorf("Judge = %+v", cfg.Judge)
	}
}

// TestLoadNormalizes covers the values Load rewrites instead of rejecting.
func TestLoadNormalizes(t *testing.T) {
	clearEnv(t)
	setRequired(t, map[string]string{
		"RUNCODES_JUDGE_URL":        "http://judge:9000/",
		"RUNCODES_S3_BUCKET_PREFIX": "-runcodes-",
	})

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Judge.URL != "http://judge:9000" {
		t.Errorf("Judge.URL = %q, want the trailing slash trimmed", cfg.Judge.URL)
	}
	if got := cfg.S3.CasesBucket(); got != "runcodes-cases" {
		t.Errorf("CasesBucket = %q, want the dashes trimmed", got)
	}
}

func TestLoadRejectsMissingValues(t *testing.T) {
	for _, key := range []string{
		"RUNCODES_API_PORT", "RUNCODES_JWT_SECRET", "RUNCODES_DB_HOST",
		"RUNCODES_DB_PORT", "RUNCODES_DB_USER", "RUNCODES_DB_PASSWORD",
		"RUNCODES_DB_NAME",
	} {
		t.Run(key, func(t *testing.T) {
			clearEnv(t)
			setRequired(t, nil)
			t.Setenv(key, "")

			_, err := Load()
			if err == nil {
				t.Fatalf("expected an error for an unset %s", key)
			}
			if !strings.Contains(err.Error(), key) {
				t.Fatalf("error does not name %s: %v", key, err)
			}
		})
	}
}

func TestLoadRejectsMalformedValues(t *testing.T) {
	tests := []struct {
		name  string
		env   map[string]string
		names string
	}{
		{
			name:  "api port is not a number",
			env:   map[string]string{"RUNCODES_API_PORT": "http"},
			names: "RUNCODES_API_PORT",
		},
		{
			name:  "api port is out of range",
			env:   map[string]string{"RUNCODES_API_PORT": "70000"},
			names: "RUNCODES_API_PORT",
		},
		{
			name:  "database port is not a number",
			env:   map[string]string{"RUNCODES_DB_PORT": "postgres"},
			names: "RUNCODES_DB_PORT",
		},
		{
			name:  "redis db is not a number",
			env:   map[string]string{"RUNCODES_REDIS_DB": "first"},
			names: "RUNCODES_REDIS_DB",
		},
		{
			name:  "negative redis db",
			env:   map[string]string{"RUNCODES_REDIS_DB": "-1"},
			names: "RUNCODES_REDIS_DB",
		},
		{
			name:  "debug mode is not a boolean",
			env:   map[string]string{"DEBUG_MODE": "yes"},
			names: "DEBUG_MODE",
		},
		{
			name:  "stale timeout is not a duration",
			env:   map[string]string{"RUNCODES_JUDGE_STALE_TIMEOUT": "15 minutes"},
			names: "RUNCODES_JUDGE_STALE_TIMEOUT",
		},
		{
			name:  "negative stale timeout",
			env:   map[string]string{"RUNCODES_JUDGE_STALE_TIMEOUT": "-1m"},
			names: "RUNCODES_JUDGE_STALE_TIMEOUT",
		},
		{
			name:  "judge url has no scheme",
			env:   map[string]string{"RUNCODES_JUDGE_URL": "judge:9000"},
			names: "RUNCODES_JUDGE_URL",
		},
		{
			name:  "judge url uses another scheme",
			env:   map[string]string{"RUNCODES_JUDGE_URL": "ftp://judge:9000"},
			names: "RUNCODES_JUDGE_URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearEnv(t)
			setRequired(t, tt.env)

			_, err := Load()
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), tt.names) {
				t.Fatalf("error does not name %s: %v", tt.names, err)
			}
		})
	}
}

// TestLoadRejectsBareDurations covers the duration format that is deliberately
// not accepted: a bare number could be seconds or minutes, so it is an error
// rather than a guess.
func TestLoadRejectsBareDurations(t *testing.T) {
	clearEnv(t)
	setRequired(t, map[string]string{"RUNCODES_JUDGE_STALE_TIMEOUT": "900"})

	_, err := Load()
	if err == nil {
		t.Fatal("expected an error for a duration without a unit")
	}
	if !strings.Contains(err.Error(), "RUNCODES_JUDGE_STALE_TIMEOUT") {
		t.Fatalf("error does not name the variable: %v", err)
	}
}

func TestDBConfigDSN(t *testing.T) {
	db := DBConfig{
		Host: "database", Port: "5432", Name: "runcodes",
		User: "runcodes", Password: "pw", SSLMode: "disable",
	}

	want := "host=database port=5432 user=runcodes password=pw dbname=runcodes sslmode=disable"
	if got := db.DSN(); got != want {
		t.Errorf("DSN = %q, want %q", got, want)
	}
}
