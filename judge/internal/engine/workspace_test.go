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

// The per-language defaults live in the images, so a judge that was not told to
// override them must not write them at all: the harness sends whatever the config
// carried, and the image's own value is what the language needs (1GB for Python,
// 64MB files for the Rust compiler, 60s to compile Go).
func TestWriteContainerConfigOmitsUnsetGlobalLimits(t *testing.T) {
	e := &Engine{cfg: &config.Config{
		DefaultCaseTimeout: 3 * time.Second,
	}}

	dir := t.TempDir()
	ws := &workspace{
		BaseDir:   dir,
		TestCases: []model.TestCase{{ID: 7, CPUTimeLimit: 5, MemUsageLimit: 1 << 20}},
	}

	if err := e.writeContainerConfig(&model.Commit{ID: 1, S3Key: "uuid/main.c"}, ws); err != nil {
		t.Fatalf("writeContainerConfig: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "container.config"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(raw)

	for _, unwanted := range []string{
		"monitor_max_fs=",
		"monitor_max_ms=",
		"compilation_timeout=",
	} {
		if strings.Contains(got, unwanted) {
			t.Errorf("container.config carries %q although the judge has no value set:\n%s", unwanted, got)
		}
	}

	// The per-case limit is the exercise's, and is still written.
	if !strings.Contains(got, "ms_7=1048576") {
		t.Errorf("the per-case limit was dropped:\n%s", got)
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

	if err := extractZip(archive, out, 1<<20, 1<<20); err != nil {
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

	if err := extractZip(archive, filepath.Join(dir, "out"), 1<<20, 1<<20); err == nil {
		t.Fatal("expected a path-traversal error")
	}
	if _, err := os.Stat(filepath.Join(dir, "evil.txt")); err == nil {
		t.Fatal("traversal entry was written outside the destination")
	}
}

func TestExtractZipRejectsOversizedEntry(t *testing.T) {
	dir := t.TempDir()
	archive := makeZip(t, dir, map[string]string{"big.c": strings.Repeat("a", 4096)})

	if err := extractZip(archive, filepath.Join(dir, "out"), 1024, 1<<20); err == nil {
		t.Fatal("expected a per-file size-limit error")
	}
}

func TestExtractZipRejectsOversizedTotal(t *testing.T) {
	dir := t.TempDir()
	archive := makeZip(t, dir, map[string]string{
		"a.c": strings.Repeat("a", 1024),
		"b.c": strings.Repeat("b", 1024),
	})

	if err := extractZip(archive, filepath.Join(dir, "out"), 1<<20, 1500); err == nil {
		t.Fatal("expected a total size-limit error")
	}
}

// TestExpectedOutputsStayOutsideTheWorkspace pins the invariant that a submission
// cannot reach its own expected answers. The workspace is bind-mounted
// read-write into the graded container, so an expected output stored inside it
// could be read, overwritten or symlinked by the code being graded.
func TestExpectedOutputsStayOutsideTheWorkspace(t *testing.T) {
	execDir := t.TempDir()
	e := &Engine{cfg: &config.Config{ExecDir: execDir}}

	expectedDir, err := e.prepareExpectedDir(7)
	if err != nil {
		t.Fatalf("prepareExpectedDir: %v", err)
	}

	ws := &workspace{
		BaseDir:     filepath.Join(execDir, workspaceName(7)),
		ExpectedDir: expectedDir,
	}

	// The container only sees BaseDir, so the expected dir must sit outside it.
	rel, err := filepath.Rel(ws.BaseDir, expectedDir)
	if err != nil || !strings.HasPrefix(rel, "..") {
		t.Fatalf("expected dir %q is inside the mounted workspace %q", expectedDir, ws.BaseDir)
	}

	// The exec directory is shared, so answers must not be group/other readable.
	fi, err := os.Stat(expectedDir)
	if err != nil {
		t.Fatal(err)
	}
	if perm := fi.Mode().Perm(); perm&0o077 != 0 {
		t.Fatalf("expected dir mode = %o, want no group or other access", perm)
	}

	if got, want := ws.expectedOutputPath(42), filepath.Join(expectedDir, "42.out"); got != want {
		t.Fatalf("expectedOutputPath = %q, want %q", got, want)
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
