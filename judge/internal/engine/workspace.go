package engine

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/runcodes-icmc/judge/internal/language"
	"github.com/runcodes-icmc/judge/internal/model"
)

// workspace is everything needed to run one commit.
type workspace struct {
	BaseDir    string
	RemoteDir  string
	SourceDir  string
	Language   *language.Language
	Extension  string
	Compilable bool
	TestCases  []model.TestCase
}

// mkdirWorld creates dir (and parents) and forces mode 0777 regardless of the
// process umask. The container runs under a user namespace where its root maps
// to a subuid, so the workspace must be world-writable for it to compile and
// write outputs (the legacy engine did the same with 0o777).
func mkdirWorld(dir string) error {
	if err := os.MkdirAll(dir, 0o777); err != nil {
		return err
	}
	return os.Chmod(dir, 0o777)
}

// prepareWorkspace creates the run directory and downloads every input.
func (e *Engine) prepareWorkspace(ctx context.Context, commit *model.Commit) (*workspace, error) {
	name := fmt.Sprintf("commit_%d", commit.ID)
	baseDir := filepath.Join(e.cfg.ExecDir, name)
	remoteDir := filepath.Join(e.cfg.ExecDirRemote, name)

	if err := os.RemoveAll(baseDir); err != nil {
		return nil, fmt.Errorf("clean workspace: %w", err)
	}
	srcDir := filepath.Join(baseDir, "src")
	if err := mkdirWorld(baseDir); err != nil {
		return nil, fmt.Errorf("create workspace: %w", err)
	}
	if err := mkdirWorld(srcDir); err != nil {
		return nil, fmt.Errorf("create source dir: %w", err)
	}

	ws := &workspace{BaseDir: baseDir, RemoteDir: remoteDir, SourceDir: srcDir}

	fname := commit.Filename()
	sourcePath := filepath.Join(srcDir, filepath.Base(fname))
	if err := e.s3.FetchCommitSource(ctx, commit.S3Key, sourcePath); err != nil {
		return nil, fmt.Errorf("download source: %w", err)
	}

	if err := e.resolveLanguage(ws, sourcePath); err != nil {
		return nil, err
	}

	if err := e.fetchCompilationFiles(ctx, commit, ws); err != nil {
		return nil, err
	}

	testCases, err := e.store.FetchTestCases(ctx, commit.RealExerciseID)
	if err != nil {
		return nil, fmt.Errorf("fetch test cases: %w", err)
	}
	ws.TestCases = testCases
	if err := e.fetchTestCases(ctx, ws); err != nil {
		return nil, err
	}

	if err := e.writeContainerConfig(commit, ws); err != nil {
		return nil, err
	}
	return ws, nil
}

// resolveLanguage derives the language from the file name, extracting archives.
func (e *Engine) resolveLanguage(ws *workspace, sourcePath string) error {
	fname := filepath.Base(sourcePath)
	ext := strings.ToLower(strings.TrimPrefix(path.Ext(fname), "."))

	if ext == "zip" {
		deduced, err := deduceArchiveLanguage(sourcePath)
		if err != nil {
			return fmt.Errorf("deduce language from archive: %w", err)
		}
		ext = deduced
		if err := extractZip(sourcePath, ws.SourceDir); err != nil {
			return fmt.Errorf("extract archive: %w", err)
		}
		ws.Language = language.FromExtension(ext)
	} else {
		ws.Language = language.FromFilename(fname)
	}

	ws.Extension = ext
	ws.Compilable = language.IsCompilable(ext)
	if ws.Language == nil && ext != "zip" {
		return fmt.Errorf("unsupported language for file %q", fname)
	}
	return nil
}

func deduceArchiveLanguage(sourcePath string) (string, error) {
	zr, err := zip.OpenReader(sourcePath)
	if err != nil {
		return "", err
	}
	defer zr.Close()
	names := make([]string, 0, len(zr.File))
	for _, f := range zr.File {
		names = append(names, f.Name)
	}
	return language.DeduceFromArchive(names)
}

func (e *Engine) fetchCompilationFiles(ctx context.Context, commit *model.Commit, ws *workspace) error {
	paths, err := e.store.FetchCompilationFiles(ctx, commit.RealExerciseID)
	if err != nil {
		return fmt.Errorf("fetch compilation files: %w", err)
	}
	for _, p := range paths {
		dest := filepath.Join(ws.SourceDir, filepath.Base(p))
		if err := e.s3.FetchCompilationFile(ctx, commit.RealExerciseID, p, dest); err != nil {
			return fmt.Errorf("download compilation file %q: %w", p, err)
		}
	}
	return nil
}

func (e *Engine) fetchTestCases(ctx context.Context, ws *workspace) error {
	for _, tc := range ws.TestCases {
		inputDest := filepath.Join(ws.BaseDir, fmt.Sprintf("%d.in", tc.ID))
		if err := e.s3.FetchCaseInput(ctx, tc.ID, inputDest); err != nil {
			return fmt.Errorf("download input of case %d: %w", tc.ID, err)
		}
		testDir := filepath.Join(ws.BaseDir, fmt.Sprintf("test_%d", tc.ID))
		if err := mkdirWorld(testDir); err != nil {
			return fmt.Errorf("create dir of case %d: %w", tc.ID, err)
		}
		if len(tc.Files) > 0 {
			if err := e.s3.FetchCaseFiles(ctx, tc.ID, tc.Files, testDir); err != nil {
				return fmt.Errorf("download files of case %d: %w", tc.ID, err)
			}
		}
	}
	return nil
}

// writeContainerConfig emits the `container.config` file the image script
// sources. Format and keys mirror the legacy engine.
func (e *Engine) writeContainerConfig(commit *model.Commit, ws *workspace) error {
	defaultCase := int(e.cfg.DefaultCaseTimeout.Seconds())
	var b strings.Builder
	fmt.Fprintf(&b, "monitor_max_fs=%d\n", e.cfg.MonitorMaxFileSize)
	fmt.Fprintf(&b, "monitor_max_ms=%d\n", e.cfg.MonitorMaxMemSize)
	fmt.Fprintf(&b, "compilation_timeout=%d\n", int(e.cfg.CompilationTimeout.Seconds()))
	fmt.Fprintf(&b, "src_file='%s'\n", shellSingleQuote(commit.Filename()))
	for _, tc := range ws.TestCases {
		timeout := tc.CPUTimeLimit
		if timeout <= 0 {
			timeout = defaultCase
		}
		fmt.Fprintf(&b, "t_%d=%d\n", tc.ID, timeout)
	}
	return os.WriteFile(filepath.Join(ws.BaseDir, "container.config"), []byte(b.String()), 0o644)
}

// shellSingleQuote escapes a value for use inside single quotes.
func shellSingleQuote(s string) string {
	return strings.ReplaceAll(s, "'", `'\''`)
}

// extractZip unpacks an archive into dest, rejecting entries that escape it.
func extractZip(archivePath, dest string) error {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer zr.Close()

	for _, f := range zr.File {
		target := filepath.Join(dest, filepath.Clean(f.Name))
		if target != dest && !strings.HasPrefix(target, dest+string(os.PathSeparator)) {
			return fmt.Errorf("archive entry %q escapes destination", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := mkdirWorld(target); err != nil {
				return err
			}
			continue
		}
		if err := mkdirWorld(filepath.Dir(target)); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o777)
		if err != nil {
			rc.Close()
			return err
		}
		if _, err := io.Copy(out, rc); err != nil {
			out.Close()
			rc.Close()
			return err
		}
		out.Close()
		rc.Close()
	}
	return nil
}
