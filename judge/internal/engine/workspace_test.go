package engine

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/runcodes-icmc/judge/internal/config"
	"github.com/runcodes-icmc/judge/internal/model"
)

func TestWriteContainerConfig(t *testing.T) {
	e := &Engine{cfg: &config.Config{
		MonitorMaxFileSize: 5 * 1024 * 1024,
		MonitorMaxMemSize:  256 * 1024 * 1024,
		CompilationTimeout: 10 * time.Second,
		DefaultCaseTimeout: 3 * time.Second,
	}}

	dir := t.TempDir()
	ws := &workspace{
		BaseDir:   dir,
		TestCases: []model.TestCase{{ID: 11, CPUTimeLimit: 5}, {ID: 12, CPUTimeLimit: 0}},
	}
	commit := &model.Commit{ID: 1, S3Key: "some/uuid/main.c"}

	if err := e.writeContainerConfig(commit, ws); err != nil {
		t.Fatalf("writeContainerConfig: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "container.config"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(raw)
	for _, want := range []string{
		"monitor_max_fs=5242880",
		"monitor_max_ms=268435456",
		"compilation_timeout=10",
		"src_file='main.c'",
		"t_11=5",
		"t_12=3", // falls back to the default case timeout
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in container.config:\n%s", want, got)
		}
	}
}

func TestShellSingleQuote(t *testing.T) {
	if got, want := shellSingleQuote("a'b"), `a'\''b`; got != want {
		t.Fatalf("shellSingleQuote = %q, want %q", got, want)
	}
	if got := shellSingleQuote("plain"); got != "plain" {
		t.Fatalf("shellSingleQuote = %q, want plain", got)
	}
}

func makeZip(t *testing.T, dir string, entries map[string]string) string {
	t.Helper()
	path := filepath.Join(dir, "archive.zip")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for name, content := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestExtractZip(t *testing.T) {
	dir := t.TempDir()
	archive := makeZip(t, dir, map[string]string{"src/main.c": "int main(){}"})
	out := filepath.Join(dir, "out")

	if err := extractZip(archive, out); err != nil {
		t.Fatalf("extractZip: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(out, "src", "main.c"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "int main(){}" {
		t.Fatalf("unexpected content: %q", content)
	}
}

func TestExtractZipRejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	archive := makeZip(t, dir, map[string]string{"../evil.txt": "boom"})

	if err := extractZip(archive, filepath.Join(dir, "out")); err == nil {
		t.Fatal("expected a path-traversal error")
	}
	if _, err := os.Stat(filepath.Join(dir, "evil.txt")); err == nil {
		t.Fatal("traversal entry was written outside the destination")
	}
}

func TestExecutionTimeout(t *testing.T) {
	e := &Engine{cfg: &config.Config{
		BaseExecTimeout:    5 * time.Second,
		DefaultCaseTimeout: 3 * time.Second,
	}}
	cases := []model.TestCase{{ID: 1, CPUTimeLimit: 4}, {ID: 2, CPUTimeLimit: 0}, {ID: 3, CPUTimeLimit: 2}}
	// base*(1+3) + 4 + default(3) + 2 = 20 + 9
	if got, want := e.executionTimeout(cases), 29*time.Second; got != want {
		t.Fatalf("executionTimeout = %s, want %s", got, want)
	}
}
