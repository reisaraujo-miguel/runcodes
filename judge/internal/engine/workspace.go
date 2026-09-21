package engine

import (
	"archive/zip"
	"context"
	"crypto/rand"
	"encoding/hex"
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
	BaseDir   string
	RemoteDir string
	SourceDir string
	// ExpectedDir holds the expected outputs downloaded from S3. It lives
	// OUTSIDE BaseDir on purpose: BaseDir is bind-mounted read-write into the
	// graded container, so anything inside it can be read, replaced or symlinked
	// by the submission. The judge reads the expected output from here to grade.
	ExpectedDir string
	// RunNonce authenticates the progress milestones the image prints. It is kept
	// with the workspace only until it has been written to the container config;
	// it never appears in the judge's log or events.
	RunNonce   string
	Language   *language.Language
	Extension  string
	Compilable bool
	TestCases  []model.TestCase
}

// newRunNonce returns the random value the image echoes back with every
// milestone.
//
// The milestones are plain log lines, so without a secret a submission could
// print "run.done" itself and send the judge off to grade whatever happens to be
// on disk. The harness reads the nonce from container.config and deletes that
// file before any submitted code runs, so the value never reaches the code being
// graded — including a Makefile, which executes during compilation.
func newRunNonce() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate run nonce: %w", err)
	}

	return hex.EncodeToString(raw[:]), nil
}

// mkdirWorld creates dir (and parents) and forces mode 0777 regardless of the
// process umask. The container runs under a user namespace where its root maps
// to a subuid, so the workspace must be world-writable for it to compile and
// write outputs.
//
// Only call this for paths the container is meant to write. Anything the judge
// alone reads stays private: see prepareWorkspace's expected dir.
func mkdirWorld(dir string) error {
	if err := os.MkdirAll(dir, 0o777); err != nil {
		return err
	}

	return os.Chmod(dir, 0o777)
}

// workspaceName is the run directory name for a commit, shared by the workspace
// and its expected-output directory so both can be found and cleaned up together.
func workspaceName(commitID int64) string {
	return fmt.Sprintf("commit_%d", commitID)
}

// prepareWorkspace creates the run directory and downloads every input.
func (e *Engine) prepareWorkspace(ctx context.Context, commit *model.Commit) (*workspace, error) {
	name := workspaceName(commit.ID)
	baseDir := filepath.Join(e.cfg.ExecDir, name)
	remoteDir := filepath.Join(e.cfg.ExecDirRemote, name)

	// Remove any existing workspace to ensure a clean state.
	if err := os.RemoveAll(baseDir); err != nil {
		return nil, fmt.Errorf("clean workspace: %w", err)
	}

	// Create the base directory for the workspace.
	if err := mkdirWorld(baseDir); err != nil {
		return nil, fmt.Errorf("create workspace: %w", err)
	}

	// Create the source directory inside the base directory.
	srcDir := filepath.Join(baseDir, "src")
	if err := mkdirWorld(srcDir); err != nil {
		return nil, fmt.Errorf("create source dir: %w", err)
	}

	expectedDir, err := e.prepareExpectedDir(commit.ID)
	if err != nil {
		return nil, err
	}

	nonce, err := newRunNonce()
	if err != nil {
		return nil, err
	}

	ws := &workspace{
		BaseDir: baseDir, RemoteDir: remoteDir, SourceDir: srcDir,
		ExpectedDir: expectedDir, RunNonce: nonce,
	}

	fname := commit.Filename()

	// Download the source file from S3 to the source directory.
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

// expectedOutputPath is where the expected output of a test case is stored: in
// the private expected directory, never in the container-visible workspace.
func (ws *workspace) expectedOutputPath(caseID int64) string {
	return filepath.Join(ws.ExpectedDir, fmt.Sprintf("%d.out", caseID))
}

// prepareExpectedDir creates the private directory that holds a run's expected
// outputs, replacing anything left over from a previous attempt.
//
// It is a sibling of the workspace, never a child: the workspace is bind-mounted
// read-write into the graded container, so an expected answer stored inside it
// could be read, overwritten or symlinked by the submission being graded. Mode
// 0700 also keeps answers away from the other users of the shared exec directory.
func (e *Engine) prepareExpectedDir(commitID int64) (string, error) {
	dir := filepath.Join(e.cfg.ExecDir, fmt.Sprintf("expected_%d", commitID))

	if err := os.RemoveAll(dir); err != nil {
		return "", fmt.Errorf("clean expected dir: %w", err)
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create expected dir: %w", err)
	}

	return dir, nil
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
		if err := extractZip(sourcePath, ws.SourceDir, e.cfg.MaxExtractFileBytes, e.cfg.MaxExtractBytes); err != nil {
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

// deduceArchiveLanguage inspects the contents of a zip archive to determine the programming language.
func deduceArchiveLanguage(sourcePath string) (string, error) {
	zr, err := zip.OpenReader(sourcePath)
	if err != nil {
		return "", err
	}

	defer zr.Close()

	// Collect the names of all files in the archive to deduce the language.
	names := make([]string, 0, len(zr.File))

	// Iterate through the files in the zip archive and append their names to the list.
	for _, f := range zr.File {
		names = append(names, f.Name)
	}

	return language.DeduceFromArchive(names)
}

// fetchCompilationFiles downloads any additional files needed for compilation from S3.
func (e *Engine) fetchCompilationFiles(ctx context.Context, commit *model.Commit, ws *workspace) error {
	paths, err := e.store.FetchCompilationFiles(ctx, commit.RealExerciseID)
	if err != nil {
		return fmt.Errorf("fetch compilation files: %w", err)
	}

	// Download each compilation file from S3 to the source directory.
	for _, p := range paths {
		dest := filepath.Join(ws.SourceDir, filepath.Base(p))
		if err := e.s3.FetchCompilationFile(ctx, commit.RealExerciseID, p, dest); err != nil {
			return fmt.Errorf("download compilation file %q: %w", p, err)
		}
	}

	return nil
}

// fetchTestCases downloads the input and any additional files for each test case from S3.
func (e *Engine) fetchTestCases(ctx context.Context, ws *workspace) error {
	// Iterate through each test case and download its input and any additional files.
	for _, tc := range ws.TestCases {
		inputDest := filepath.Join(ws.BaseDir, fmt.Sprintf("%d.in", tc.ID))
		if err := e.s3.FetchCaseInput(ctx, tc.ID, inputDest); err != nil {
			return fmt.Errorf("download input of case %d: %w", tc.ID, err)
		}

		testDir := filepath.Join(ws.BaseDir, fmt.Sprintf("test_%d", tc.ID))
		if err := mkdirWorld(testDir); err != nil {
			return fmt.Errorf("create dir of case %d: %w", tc.ID, err)
		}

		// If the test case has additional files, download them from S3 to the test directory.
		if len(tc.Files) > 0 {
			if err := e.s3.FetchCaseFiles(ctx, tc.ID, tc.Files, testDir); err != nil {
				return fmt.Errorf("download files of case %d: %w", tc.ID, err)
			}
		}
	}

	return nil
}

// writeContainerConfig writes the container configuration file: the milestone
// nonce, the global monitor settings and the per-case limits and timeouts.
//
// The values are read by the image's harness with bash's `source`, so every
// string is quoted as a single-quoted shell word and anything that could start a
// new statement is rejected rather than escaped.
func (e *Engine) writeContainerConfig(commit *model.Commit, ws *workspace) error {
	defaultCase := int(e.cfg.DefaultCaseTimeout.Seconds())

	var b strings.Builder

	// The global limits are only written when the operator configured them: every
	// image carries its own per-language default (`monitor_max_ms` is 1GB for the
	// Python image, and the JVM images set no limit at all), and the harness lets a
	// value the judge sent win over them. Writing the judge's default for every
	// language would flatten those defaults — capping the Rust or Go compiler at
	// 256MB, for one. 0 therefore means "use the image's default", and an explicit
	// value is an override for the whole deployment. `compilation_timeout` works the
	// same way, and `JUDGE_COMPILATION_WAIT` is what bounds the judge's own patience
	// (see config), so the two are independent numbers.
	if e.cfg.MonitorMaxFileSize > 0 {
		fmt.Fprintf(&b, "monitor_max_fs=%d\n", e.cfg.MonitorMaxFileSize)
	}
	if e.cfg.MonitorMaxMemSize > 0 {
		fmt.Fprintf(&b, "monitor_max_ms=%d\n", e.cfg.MonitorMaxMemSize)
	}
	if e.cfg.CompilationTimeout > 0 {
		fmt.Fprintf(&b, "compilation_timeout=%d\n", int(e.cfg.CompilationTimeout.Seconds()))
	}
	fmt.Fprintf(&b, "run_nonce='%s'\n", shellSingleQuote(ws.RunNonce))

	srcFile, err := shellWord(commit.Filename())
	if err != nil {
		return err
	}
	fmt.Fprintf(&b, "src_file='%s'\n", srcFile)

	// Write the per-case settings, omitting the ones the exercise leaves unset so
	// the image's own defaults apply. A limit the exercise did not author is not
	// the same as a limit of zero.
	for _, tc := range ws.TestCases {
		timeout := tc.CPUTimeLimit
		if timeout <= 0 {
			timeout = defaultCase
		}
		fmt.Fprintf(&b, "t_%d=%d\n", tc.ID, timeout)

		if tc.MemUsageLimit > 0 {
			fmt.Fprintf(&b, "ms_%d=%d\n", tc.ID, tc.MemUsageLimit)
		}
		if tc.FileSizeLimit > 0 {
			fmt.Fprintf(&b, "fs_%d=%d\n", tc.ID, tc.FileSizeLimit)
		}
		if tc.StackLimit > 0 {
			fmt.Fprintf(&b, "stack_%d=%d\n", tc.ID, tc.StackLimit)
		}
	}

	return os.WriteFile(filepath.Join(ws.BaseDir, "container.config"), []byte(b.String()), 0o644)
}

// shellSingleQuote escapes a value for use inside single quotes.
func shellSingleQuote(s string) string {
	return strings.ReplaceAll(s, "'", `'\''`)
}

// shellWord validates a value for the single-quoted assignment the harness reads.
//
// A line break would end the assignment and turn the rest of the value into an
// instruction for the harness, which runs as the container's root: the file name
// comes from the submitted object's key, so it is rejected rather than escaped.
func shellWord(value string) (string, error) {
	if strings.ContainsAny(value, "\n\r\x00") {
		return "", fmt.Errorf("invalid file name %q for the container config", value)
	}

	return shellSingleQuote(value), nil
}

// extractZip unpacks an archive into dest, rejecting entries that escape it or
// whose expanded size exceeds the configured bounds. The bounds stop a zip bomb
// from filling the shared execution directory: the per-entry and the running
// total are checked both against the size declared in the archive header and
// against the bytes actually written, since a header can lie.
func extractZip(archivePath, dest string, maxFileBytes, maxTotalBytes int64) error {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}

	defer zr.Close()

	var total int64

	// Iterate through each file in the zip archive and extract it to the destination directory.
	for _, f := range zr.File {
		target := filepath.Join(dest, filepath.Clean(f.Name))

		// Check if the target path escapes the destination directory. If it does, return an error.
		if target != dest && !strings.HasPrefix(target, dest+string(os.PathSeparator)) {
			return fmt.Errorf("archive entry %q escapes destination", f.Name)
		}

		// Create the target directory if it doesn't exist.
		if f.FileInfo().IsDir() {
			if err := mkdirWorld(target); err != nil {
				return err
			}

			continue
		}

		// Reject entries whose declared uncompressed size alone already exceeds
		// the per-entry limit or the remaining total budget.
		if f.UncompressedSize64 > uint64(maxFileBytes) {
			return fmt.Errorf("archive entry %q exceeds the per-file size limit", f.Name)
		}
		if total+int64(f.UncompressedSize64) > maxTotalBytes {
			return fmt.Errorf("archive expands beyond the total size limit")
		}

		// Create the parent directory for the target file if it doesn't exist.
		if err := mkdirWorld(filepath.Dir(target)); err != nil {
			return err
		}

		// Open the zip entry for reading.
		rc, err := f.Open()
		if err != nil {
			return err
		}

		// Create the target file with world-writable permissions.
		out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o777)
		if err != nil {
			rc.Close()
			return err
		}

		// Bound the copy by both the per-entry limit and the remaining total
		// budget, reading one byte past the limit so an oversized entry is
		// detected even when its header understates the real size.
		limit := maxFileBytes
		if remaining := maxTotalBytes - total; remaining < limit {
			limit = remaining
		}

		written, copyErr := io.Copy(out, io.LimitReader(rc, limit+1))
		out.Close()
		rc.Close()

		if copyErr != nil {
			return copyErr
		}
		if written > limit {
			return fmt.Errorf("archive entry %q exceeds the size limit", f.Name)
		}

		total += written
	}

	return nil
}
