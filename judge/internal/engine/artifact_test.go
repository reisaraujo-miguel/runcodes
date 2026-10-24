package engine

import (
	"archive/zip"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/runcodes-icmc/judge/internal/config"
	"github.com/runcodes-icmc/judge/internal/model"
)

// newArtifactEngine builds the minimal engine state buildArtifact needs.
func newArtifactEngine(t *testing.T) (*Engine, *workspace) {
	t.Helper()

	baseDir := t.TempDir()
	return &Engine{
		cfg: &config.Config{
			ExecDir:          t.TempDir(),
			MaxArtifactBytes: 1 << 20,
		},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}, &workspace{
		BaseDir:   baseDir,
		SourceDir: filepath.Join(baseDir, "src"),
	}
}

func outputDir(t *testing.T, baseDir string) string {
	t.Helper()

	dir := filepath.Join(baseDir, "outputfiles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	return dir
}

func assertNoArtifact(t *testing.T, engine *Engine, commitID int64) {
	t.Helper()

	if _, err := os.Stat(engine.ArtifactPath(commitID)); !os.IsNotExist(err) {
		t.Fatalf("expected no artifact at %s, stat error: %v", engine.ArtifactPath(commitID), err)
	}
}

func readZipEntries(t *testing.T, path string) map[string]string {
	t.Helper()

	reader, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("zip.OpenReader: %v", err)
	}
	defer reader.Close()

	entries := make(map[string]string, len(reader.File))
	for _, file := range reader.File {
		rc, err := file.Open()
		if err != nil {
			t.Fatalf("open %s: %v", file.Name, err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatalf("read %s: %v", file.Name, err)
		}
		entries[file.Name] = string(data)
	}

	return entries
}

func TestBuildArtifactSkipsMissingOutputDir(t *testing.T) {
	engine, ws := newArtifactEngine(t)

	path, err := engine.buildArtifact(1, ws)
	if err != nil {
		t.Fatalf("buildArtifact: %v", err)
	}
	if path != "" {
		t.Fatalf("path = %q, want empty", path)
	}
	assertNoArtifact(t, engine, 1)
}

func TestBuildArtifactSkipsEmptyOutputDir(t *testing.T) {
	engine, ws := newArtifactEngine(t)
	outputDir(t, ws.BaseDir)

	path, err := engine.buildArtifact(2, ws)
	if err != nil {
		t.Fatalf("buildArtifact: %v", err)
	}
	if path != "" {
		t.Fatalf("path = %q, want empty", path)
	}
	assertNoArtifact(t, engine, 2)
}

func TestBuildArtifactSkipsDirectoriesOnly(t *testing.T) {
	engine, ws := newArtifactEngine(t)
	dir := outputDir(t, ws.BaseDir)
	if err := os.Mkdir(filepath.Join(dir, "nested"), 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	path, err := engine.buildArtifact(3, ws)
	if err != nil {
		t.Fatalf("buildArtifact: %v", err)
	}
	if path != "" {
		t.Fatalf("path = %q, want empty", path)
	}
	assertNoArtifact(t, engine, 3)
}

func TestBuildArtifactArchivesGeneratedFiles(t *testing.T) {
	engine, ws := newArtifactEngine(t)
	dir := outputDir(t, ws.BaseDir)
	if err := os.WriteFile(filepath.Join(dir, "result.txt"), []byte("answer"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "empty.txt"), nil, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	path, err := engine.buildArtifact(4, ws)
	if err != nil {
		t.Fatalf("buildArtifact: %v", err)
	}
	if path != engine.ArtifactPath(4) {
		t.Fatalf("path = %q, want %q", path, engine.ArtifactPath(4))
	}

	entries := readZipEntries(t, path)

	if got := entries["result.txt"]; got != "answer" {
		t.Fatalf("result.txt = %q, want answer", got)
	}
	if got, ok := entries["empty.txt"]; !ok || got != "" {
		t.Fatalf("empty.txt = %q (present=%v), want an empty entry", got, ok)
	}
}

func TestIsInternalOutputFile(t *testing.T) {
	tests := []struct {
		name     string
		internal bool
	}{
		{"1.output", true},
		{"12.error", true},
		{"3.monitor_out", true},
		{"4.monitor_err", true},
		{"compilation.out", true},
		{"compilation.err", true},
		{"compilation.monitor_out", true},
		{"compilation.monitor_err", true},
		{"nested/1.output", true},
		{"result.output", false},
		{"notes.error", false},
		{"compilation_notes.txt", false},
		{"1.txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isInternalOutputFile(tt.name); got != tt.internal {
				t.Fatalf("isInternalOutputFile(%q) = %v, want %v", tt.name, got, tt.internal)
			}
		})
	}
}

func TestBuildArtifactIgnoresInternalFiles(t *testing.T) {
	engine, ws := newArtifactEngine(t)
	dir := outputDir(t, ws.BaseDir)

	internal := []string{
		"1.output",
		"1.error",
		"1.monitor_out",
		"1.monitor_err",
		"compilation.out",
		"compilation.err",
		"compilation.monitor_out",
		"compilation.monitor_err",
	}
	for _, name := range internal {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("internal"), 0o644); err != nil {
			t.Fatalf("WriteFile %s: %v", name, err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "result.txt"), []byte("user"), 0o644); err != nil {
		t.Fatalf("WriteFile result.txt: %v", err)
	}

	path, err := engine.buildArtifact(5, ws)
	if err != nil {
		t.Fatalf("buildArtifact: %v", err)
	}
	entries := readZipEntries(t, path)
	if len(entries) != 1 || entries["result.txt"] != "user" {
		t.Fatalf("entries = %v, want only result.txt", entries)
	}
}

func TestBuildArtifactSkipsInternalOnlyFiles(t *testing.T) {
	engine, ws := newArtifactEngine(t)
	dir := outputDir(t, ws.BaseDir)

	for _, name := range []string{"1.output", "2.monitor_out", "compilation.err"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("internal"), 0o644); err != nil {
			t.Fatalf("WriteFile %s: %v", name, err)
		}
	}

	path, err := engine.buildArtifact(6, ws)
	if err != nil {
		t.Fatalf("buildArtifact: %v", err)
	}
	if path != "" {
		t.Fatalf("path = %q, want empty", path)
	}
	assertNoArtifact(t, engine, 6)
}

func TestBuildArtifactCollectsTestDirectoryFiles(t *testing.T) {
	engine, ws := newArtifactEngine(t)

	write := func(rel, data string) {
		t.Helper()
		path := filepath.Join(ws.BaseDir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("MkdirAll %s: %v", rel, err)
		}
		if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
			t.Fatalf("WriteFile %s: %v", rel, err)
		}
	}

	// Source files are copied into each test directory by the runner; they are
	// inputs, not output.
	write("src/main.py", "print('hello')\n")
	write("src/lib/helper.py", "VALUE = 1\n")
	write("test_11/main.py", "print('hello')\n")
	write("test_11/lib/helper.py", "VALUE = 1\n")

	// A file supplied by the test case is also an input.
	write("test_11/data.txt", "case input")
	ws.TestCases = []model.TestCase{{ID: 11, Files: []string{"data.txt"}}}

	// Files the submission creates in its working directory.
	write("test_11/hello.txt", "hello")
	write("test_11/nested/result.txt", "nested")

	// A file written explicitly to outputfiles, next to an internal report.
	write("outputfiles/1.output", "internal")
	write("outputfiles/explicit.txt", "user")

	path, err := engine.buildArtifact(7, ws)
	if err != nil {
		t.Fatalf("buildArtifact: %v", err)
	}

	entries := readZipEntries(t, path)
	want := map[string]string{
		"explicit.txt":              "user",
		"test_11/hello.txt":         "hello",
		"test_11/nested/result.txt": "nested",
	}
	if len(entries) != len(want) {
		t.Fatalf("entries = %v, want %v", entries, want)
	}
	for name, content := range want {
		if got := entries[name]; got != content {
			t.Fatalf("%s = %q, want %q (entries: %v)", name, got, content, entries)
		}
	}
}
