package engine

/*
This file publishes the files a submission generated.

The runner collects them inside the workspace — `outputfiles/case_<id>/...` for a
test case, `outputfiles/...` for a file written straight into the output
directory — and the judge uploads each case's files as soon as that case
finishes, then deletes the local copy. The workspace is therefore bounded by one
case instead of growing with the suite, and the backend serves the files from
object storage without the judge being online.

What the submission wrote is untrusted: only regular files are read (never a
symlink, which would otherwise publish the contents of any file the judge can
read). Each file is bounded by its case's file-size limit, and every file of the
run shares MaxCommitArtifactBytes, so a suite cannot multiply the per-case limit
into an unbounded amount of object storage.
*/

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/runcodes-icmc/judge/internal/events"
	"github.com/runcodes-icmc/judge/internal/model"
)

// generatedFileStore is the slice of object storage the run needs in order to
// publish what a submission produced. It exists so the upload path can be
// exercised without SeaweedFS.
type generatedFileStore interface {
	// UploadGeneratedFile stores src under the commit's output namespace at rel
	// and returns how many bytes it stored.
	UploadGeneratedFile(ctx context.Context, commitID int64, rel, src string) (int64, error)
}

// errNoCollectionDir reports that a finished case left no collection directory.
// The harness creates one for every case, so its absence means the image running
// the case predates the collection step: the files, if any, are already gone with
// the case's scratch directory and cannot be published.
var errNoCollectionDir = fmt.Errorf("the runner image did not collect generated files")

// generatedFile is one file a submission produced, as found in the workspace.
type generatedFile struct {
	// Path is slash-separated and relative to the directory it was found in.
	Path string
	Size int64
}

/*
artifactBudget caps how many bytes one commit may publish. Every case's files
share it, so a suite cannot multiply the per-case file-size limit into an
unbounded amount of object storage. It is safe for the publisher's workers to
use concurrently: the bytes are reserved before an upload and released when the
upload fails.
*/
type artifactBudget struct {
	limit     int64
	used      atomic.Int64
	exhausted atomic.Bool
}

// newArtifactBudget returns the budget of one run. A zero limit disables it.
func newArtifactBudget(limit int64) *artifactBudget {
	return &artifactBudget{limit: limit}
}

// reserve claims n bytes, reporting whether the commit had room left for them.
// A refusal marks the budget exhausted, which lets the publisher stop announcing
// uploads it already knows will never fit.
func (b *artifactBudget) reserve(n int64) bool {
	if b == nil || b.limit <= 0 || n <= 0 {
		return true
	}

	for {
		used := b.used.Load()
		if used > b.limit-n {
			b.exhausted.Store(true)
			return false
		}
		if b.used.CompareAndSwap(used, used+n) {
			return true
		}
	}
}

// release gives back bytes reserved for an upload that did not reach storage.
// If the released bytes leave room again, the budget is no longer exhausted: an
// upload error is not the same thing as the commit having published its fill.
func (b *artifactBudget) release(n int64) {
	if b == nil || b.limit <= 0 || n <= 0 {
		return
	}

	if b.used.Add(-n) < b.limit {
		b.exhausted.Store(false)
	}
}

// isExhausted reports whether the commit has published its fill or a
// reservation has already failed. The remaining cases of the run are then
// refused with that reason instead of being queued for an upload that cannot
// fit.
func (b *artifactBudget) isExhausted() bool {
	if b == nil || b.limit <= 0 {
		return false
	}

	return b.exhausted.Load() || b.used.Load() >= b.limit
}

// caseOutputDir is the directory the runner collects one test case's files in.
func caseOutputDir(caseID int64) string {
	return "case_" + strconv.FormatInt(caseID, 10)
}

// isCaseOutputDir reports whether name is one of the runner's per-case
// collection directories. Those are uploaded with their case, so the pass over
// the output directory's root must not descend into them and store the same
// files twice.
func isCaseOutputDir(name string) bool {
	if !strings.HasPrefix(name, "case_") {
		return false
	}

	_, err := strconv.ParseInt(strings.TrimPrefix(name, "case_"), 10, 64)
	return err == nil
}

/*
pendingOutputDir is where a case's collected files wait for upload. It lives
outside the run's workspace on purpose: a queued file must not count against the
run's workspace quota, which is sized for the program's own work, not for the
judge's upload backlog. The sweeper reclaims the directory if a run dies before
its uploads finish.
*/
func (e *Engine) pendingOutputDir(commitID, caseID int64) string {
	return filepath.Join(
		e.cfg.ExecDir,
		"pending_"+strconv.FormatInt(commitID, 10),
		"case_"+strconv.FormatInt(caseID, 10),
	)
}

/*
collectGeneratedFiles lists the regular files a submission produced under dir,
recursively, with their paths relative to dir.

Directories are skipped when skipDir accepts their name, which is how the root
pass avoids the per-case directories. Symbolic links and other special files are
not followed and not returned: the workspace is written by untrusted code, and
following a link would read a file outside it.
*/
func collectGeneratedFiles(
	dir string, skipDir func(name string) bool,
) ([]generatedFile, error) {
	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	files := make([]generatedFile, 0, 8)
	err := filepath.Walk(dir, func(p string, fi os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if fi.IsDir() {
			if p != dir && skipDir != nil && skipDir(fi.Name()) {
				return filepath.SkipDir
			}
			return nil
		}

		if !fi.Mode().IsRegular() {
			return nil
		}

		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}

		files = append(files, generatedFile{
			Path: filepath.ToSlash(rel),
			Size: fi.Size(),
		})

		return nil
	})
	if err != nil {
		return nil, err
	}

	return files, nil
}

// publishRequest describes one directory of generated files to publish.
type publishRequest struct {
	// Dir is the absolute directory to collect from.
	Dir string
	// Prefix is prepended to every stored path, so a file keeps the workspace
	// location it was collected from.
	Prefix string
	// TestCaseID names the case the files came from, for the progress event the
	// client shows while the upload runs.
	TestCaseID int64
	// MaxFileBytes bounds one published file; 0 publishes any size.
	MaxFileBytes int64
	// SkipDir, when set, skips a directory by name, without descending.
	SkipDir func(name string) bool
	// SkipFile, when set, skips a file by its collected path.
	SkipFile func(rel string) bool
	// Budget, when set, is the bytes-each-case-shares cap of the commit. It is
	// the same object for every request of one run.
	Budget *artifactBudget
}

/*
publishGeneratedFiles uploads the files described by req and then removes the
request's directory. It returns how many files were uploaded, how many uploads
failed, and whether the commit's artifact budget refused at least one file.

Each file is bounded on its own by the limit of the case it belongs to, and the
run's files also share the commit's budget (req.Budget): the per-case bound alone
would let a suite of cases publish an unbounded total. The budget is only a
ceiling, so a case with a large file cannot make the following cases' files
unpublishable until the commit's total is actually exhausted. A file that cannot
be uploaded is logged and skipped: the run's results do not depend on an
artifact that could not be published.
*/
func (e *Engine) publishGeneratedFiles(
	ctx context.Context, commitID int64, req publishRequest,
) (uploaded, failed int, budgetExceeded bool, err error) {
	if e.outputs == nil {
		return 0, 0, false, fmt.Errorf("no object storage configured for generated files")
	}

	files, err := collectGeneratedFiles(req.Dir, req.SkipDir)
	if err != nil {
		return 0, 0, false, err
	}

	for _, file := range files {
		if req.SkipFile != nil && req.SkipFile(file.Path) {
			continue
		}
		if req.MaxFileBytes > 0 && file.Size > req.MaxFileBytes {
			e.logger.Warn("generated file is larger than its case allows; not published",
				"commit_id", commitID,
				"path", req.Prefix+"/"+file.Path,
				"size_bytes", file.Size,
				"limit_bytes", req.MaxFileBytes,
			)
			continue
		}
		if !req.Budget.reserve(file.Size) {
			budgetExceeded = true
			e.logger.Warn("generated files exceeded the commit's budget; not published",
				"commit_id", commitID,
				"path", req.Prefix+"/"+file.Path,
				"size_bytes", file.Size,
				"budget_bytes", req.Budget.limit,
			)
			continue
		}

		rel := file.Path
		if req.Prefix != "" {
			rel = req.Prefix + "/" + file.Path
		}

		size, err := e.outputs.UploadGeneratedFile(
			ctx, commitID, rel,
			filepath.Join(req.Dir, filepath.FromSlash(file.Path)),
		)
		if err != nil {
			req.Budget.release(file.Size)
			e.logger.Warn("could not upload a generated file",
				"commit_id", commitID,
				"path", rel,
				"error", err,
			)
			failed++
			continue
		}

		_ = size
		uploaded++
	}

	// The files are in storage now, so the local copy only costs workspace. It is
	// removed even when an upload failed: grading already read what it needs from
	// the workspace root, and the directory must not leak into the next case's
	// accounting.
	if err := os.RemoveAll(req.Dir); err != nil {
		e.logger.Warn("could not remove collected generated files",
			"commit_id", commitID, "dir", req.Dir, "error", err,
		)
	}

	return uploaded, failed, budgetExceeded, nil
}

/*
casePublishRequest builds the upload for one finished test case and reports
whether there is anything to publish.

A case interrupted by a limit or a signal publishes nothing: the program was
killed mid-write, so what it left is a truncated file — at the very size that
killed it for the file-size limit — and offering it for download would present an
unfinished output as the program's result. Its collection directory is removed
here.

A missing collection directory means the image running the case predates the
collection step; that is reported as errNoCollectionDir.
*/
func (e *Engine) casePublishRequest(
	testCase model.TestCase, result model.CaseResult, ws *workspace,
) (publishRequest, bool, error) {
	dir := filepath.Join(ws.BaseDir, "outputfiles", caseOutputDir(testCase.ID))
	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			return publishRequest{}, false, errNoCollectionDir
		}
		return publishRequest{}, false, err
	}

	// The harness creates the directory for every case, so an empty one means the
	// program wrote nothing: there is no upload to announce or wait for.
	entries, err := os.ReadDir(dir)
	if err != nil {
		return publishRequest{}, false, err
	}
	if len(entries) == 0 {
		return publishRequest{}, false, nil
	}

	if result.Status == model.CaseKilledBySignal {
		if err := os.RemoveAll(dir); err != nil {
			e.logger.Warn("could not remove an interrupted case's generated files",
				"commit_id", result.TestCaseID, "dir", dir, "error", err,
			)
		}

		return publishRequest{}, false, nil
	}

	return publishRequest{
		Dir:          dir,
		Prefix:       caseOutputPrefix(testCase),
		TestCaseID:   testCase.ID,
		MaxFileBytes: e.caseFileLimit(testCase),
		Budget:       ws.ArtifactBudget,
	}, true, nil
}

/*
caseOutputPrefix is the stored directory for one case's files. It uses the
case's position inside the exercise, not its platform-wide id, so the files a
student downloads are named test_1 .. test_n consistently with the results
panel. A case built without a number (tests and older callers) falls back to its
id.
*/
func caseOutputPrefix(testCase model.TestCase) string {
	number := testCase.Number
	if number <= 0 {
		number = int(testCase.ID)
	}

	return "test_" + strconv.Itoa(number)
}

/*
queueCaseOutputs decides what to publish for a finished case and hands the upload
to the background publisher. The decision stays on the log-stream goroutine
because it inspects the workspace; the transfer does not, so reading the next
milestone never waits for a large file.
*/
func (e *Engine) queueCaseOutputs(
	publisher *outputPublisher, commitID int64, testCase model.TestCase,
	result model.CaseResult, ws *workspace,
) error {
	if !testCase.ShowUserOutput {
		// The author did not release this case's output, so the backend would hide
		// its files anyway. Not uploading them saves the transfer and keeps the
		// client from announcing a download that will never exist.
		dir := filepath.Join(ws.BaseDir, "outputfiles", caseOutputDir(testCase.ID))
		if err := os.RemoveAll(dir); err != nil {
			e.logger.Warn("could not remove a hidden case's generated files",
				"commit_id", commitID, "dir", dir, "error", err,
			)
		}

		return nil
	}

	req, ok, err := e.casePublishRequest(testCase, result, ws)
	if err != nil || !ok {
		return err
	}

	// A previous case already used the commit's whole budget. Queueing this
	// upload would only make the client wait for files that cannot be stored, so
	// say now why they are unavailable and drop them.
	if req.Budget.isExhausted() {
		if err := os.RemoveAll(req.Dir); err != nil {
			e.logger.Warn("could not remove files past the commit's budget",
				"commit_id", commitID, "dir", req.Dir, "error", err,
			)
		}
		publisher.AnnounceBudgetExceeded(testCase.ID, req.Budget.limit)
		return nil
	}

	// Move the files out of the run's workspace before queueing: a rename on the
	// same filesystem is instant, and the queued bytes then stop competing with the
	// program for the run's workspace quota. A failed rename is not fatal; the
	// upload then reads from the workspace, exactly as before.
	if staged := e.pendingOutputDir(commitID, testCase.ID); staged != req.Dir {
		if err := os.MkdirAll(filepath.Dir(staged), 0o777); err != nil {
			e.logger.Warn("could not create the pending upload directory",
				"commit_id", commitID, "dir", filepath.Dir(staged), "error", err,
			)
		} else if err := os.Rename(req.Dir, staged); err != nil {
			e.logger.Warn("could not move a case's files out of the workspace",
				"commit_id", commitID, "case_id", testCase.ID, "error", err,
			)
		} else {
			req.Dir = staged
		}
	}

	// Announce before queueing: the client shows an upload placeholder where the
	// download link will appear. The publisher drains before the terminal event,
	// so the ready or failed event always reaches the client.
	publisher.AnnouncePending(testCase.ID)

	if publisher.Submit(req) {
		return nil
	}

	// The upload was refused: the queue is full (storage is slower than the suite)
	// or the store failed repeatedly. The placeholder is cleared with the reason,
	// the files are dropped and the run continues.
	publisher.AnnounceFailedWithReason(testCase.ID,
		events.ArtifactReasonStorageUnavailable, 0)
	e.logger.Debug("dropping a finished case's generated files",
		"commit_id", commitID, "test_case_id", testCase.ID,
	)
	if err := os.RemoveAll(req.Dir); err != nil {
		e.logger.Warn("could not remove the dropped case's generated files",
			"commit_id", commitID, "dir", req.Dir, "error", err,
		)
	}

	return nil
}

/*
caseFileLimit is the largest file this case may publish: the case's own file-size
limit, which the container also enforced (RLIMIT_FSIZE), so a file that reaches us
should always fit. A case configured without a limit falls back to the platform's
single-file ceiling, which keeps one run from publishing an unbounded file when
the author asked for no limit.
*/
func (e *Engine) caseFileLimit(testCase model.TestCase) int64 {
	if testCase.FileSizeLimit > 0 {
		return testCase.FileSizeLimit
	}

	return e.cfg.MaxArtifactBytes
}

/*
publishRootOutputs publishes the files a submission wrote straight into the
runner's output directory, after the per-case directories are gone. The runner
also copies its own per-case reports there (`<case id>.output`, `<case id>.error`,
the monitor report and the compilation metadata); those are bookkeeping, not the
submission's files, and are filtered out.
*/
func (e *Engine) publishRootOutputs(
	ctx context.Context, commitID int64, ws *workspace,
) (int, error) {
	req := publishRequest{
		Dir:    filepath.Join(ws.BaseDir, "outputfiles"),
		Prefix: "outputfiles",
		// These files belong to no case, so no case limit applies to them.
		MaxFileBytes: e.cfg.MaxArtifactBytes,
		// Per-case directories were uploaded with their case.
		SkipDir: isCaseOutputDir,
		SkipFile: func(rel string) bool {
			return isInternalOutputFile(filepath.Base(rel))
		},
		Budget: ws.ArtifactBudget,
	}

	uploaded, _, _, err := e.publishGeneratedFiles(ctx, commitID, req)
	return uploaded, err
}

/*
outputPublisher uploads one run's generated files away from the log-stream
goroutine. Reading the container's output is what drives the run forward: while
it uploads, the judge is not reading milestones, so a slow upload would delay the
next case's grading and its results. Publishing the files in the background keeps
the stream moving, and the files of one case are uploaded while the following
cases run.
*/
type outputPublisher struct {
	engine   *Engine
	ctx      context.Context
	commitID int64
	tasks    chan publishRequest
	wg       sync.WaitGroup
	// files counts the uploaded files across all workers.
	files atomic.Int64
	// failures counts consecutive upload failures, and degraded stops trying once
	// they cross the limit.
	failures atomic.Int32
	degraded atomic.Bool
	// mu guards tasks and closed: Submit and Close run on different goroutines and
	// sending on a closed channel panics.
	mu     sync.Mutex
	closed bool
	// closeOnce makes Close idempotent: the run closes the publisher before its
	// terminal event and again from the deferred cleanup.
	closeOnce sync.Once
}

// outputPublishWorkers is how many uploads run at once. A suite of cases that each
// write at their file-size limit produces data faster than one upload can store
// it, and the pipeline has to keep up with the container or the log stream ends up
// waiting for disk. Four uploads overlap a large file with the next cases without
// saturating the judge's network.
const outputPublishWorkers = 4

// outputPublishQueue bounds how many cases wait for an upload. A queued case's
// files live in the pending area outside the run's workspace, so the queue is
// sized for a burst (the tail of a suite whose cases finish faster than the store
// accepts them), not by the workspace quota. A deeper backlog is refused rather
// than held: the download is lost, the run is not delayed.
const outputPublishQueue = 8

// outputPublishFailureLimit is how many uploads may fail in a row before the run
// stops trying. A store that is down would otherwise cost one failed upload per
// case, each with its own retries, and delay the end of the run by seconds per
// test. Past the limit the remaining cases are simply not published.
const outputPublishFailureLimit = 3

// newOutputPublisher starts the upload workers and returns a handle to them.
func newOutputPublisher(
	ctx context.Context, engine *Engine, commitID int64,
) *outputPublisher {
	p := &outputPublisher{
		engine:   engine,
		ctx:      ctx,
		commitID: commitID,
		tasks:    make(chan publishRequest, outputPublishQueue),
	}

	p.wg.Add(outputPublishWorkers)
	for range outputPublishWorkers {
		go p.worker()
	}

	return p
}

// worker drains the queue until the publisher is closed.
func (p *outputPublisher) worker() {
	defer p.wg.Done()

	for req := range p.tasks {
		uploaded, failed, budgetExceeded, err := p.engine.publishGeneratedFiles(p.ctx, p.commitID, req)
		if err != nil {
			p.engine.logger.Warn("could not publish generated files",
				"commit_id", p.commitID,
				"dir", req.Dir,
				"error", err,
			)
			p.recordFailure(1)
			// The client was told to wait for this case's files, so it must be
			// told the upload will not happen.
			p.engine.hub.ArtifactCaseFailed(p.commitID, req.TestCaseID,
				events.ArtifactReasonUploadFailed, 0, "output", p.listingURL())
			continue
		}
		if budgetExceeded && uploaded == 0 {
			// No byte of this case fit the commit's remaining budget. The
			// placeholder becomes the reason, and the next cases are refused
			// before they are queued.
			limit := int64(0)
			if req.Budget != nil {
				limit = req.Budget.limit
			}
			p.engine.hub.ArtifactCaseFailed(p.commitID, req.TestCaseID,
				events.ArtifactReasonBudgetExceeded, limit, "output", p.listingURL())
			continue
		}
		if failed > 0 {
			p.recordFailure(int32(failed))
			p.engine.hub.ArtifactCaseFailed(p.commitID, req.TestCaseID,
				events.ArtifactReasonUploadFailed, 0, "output", p.listingURL())
			continue
		}
		p.failures.Store(0)
		if uploaded == 0 {
			// Nothing was stored (every file was larger than the case allows, for
			// example), so the placeholder must not keep spinning.
			p.engine.hub.ArtifactCaseFailed(p.commitID, req.TestCaseID,
				events.ArtifactReasonFileTooLarge, 0, "output", p.listingURL())
			continue
		}

		p.files.Add(int64(uploaded))

		// Tell the client this case's files are stored: it replaces the upload
		// placeholder with the download link. The event is sent per case, not once
		// per run, so each link appears as soon as its own upload finishes.
		p.engine.hub.ArtifactCase(p.commitID, req.TestCaseID,
			events.ArtifactReady, "output", p.listingURL())
	}
}

// recordFailure counts a failed upload and stops publishing for the run once the
// failures pile up.
func (p *outputPublisher) recordFailure(count int32) {
	if p.failures.Add(count) < outputPublishFailureLimit {
		return
	}

	if p.degraded.CompareAndSwap(false, true) {
		p.engine.logger.Warn(
			"object storage is failing; generated files are no longer published for this run",
			"commit_id", p.commitID,
			"failures", outputPublishFailureLimit,
		)
	}
}

// AnnouncePending tells the client that this case's files are being uploaded, so
// it renders a placeholder where the download link will appear.
func (p *outputPublisher) AnnouncePending(testCaseID int64) {
	p.engine.hub.ArtifactCase(p.commitID, testCaseID,
		events.ArtifactPending, "output", p.listingURL())
}

// AnnounceFailedWithReason tells the client that a case's files will not be
// published, and why, so its upload placeholder does not stay on screen and the
// panel can explain the missing download.
func (p *outputPublisher) AnnounceFailedWithReason(
	testCaseID int64, reason string, limitBytes int64,
) {
	p.engine.hub.ArtifactCaseFailed(p.commitID, testCaseID,
		reason, limitBytes, "output", p.listingURL())
}

// AnnounceBudgetExceeded announces that this case's files did not fit the
// commit's total artifact budget.
func (p *outputPublisher) AnnounceBudgetExceeded(testCaseID, limitBytes int64) {
	p.AnnounceFailedWithReason(testCaseID,
		events.ArtifactReasonBudgetExceeded, limitBytes)
}

// listingURL is where the client reads the commit's generated files.
func (p *outputPublisher) listingURL() string {
	return fmt.Sprintf("/api/v1/submissions/%d/outputfiles", p.commitID)
}

// Degraded reports whether the run stopped trying to publish generated files.
func (p *outputPublisher) Degraded() bool {
	return p.degraded.Load()
}

/*
Submit queues one case's directory for upload and reports whether it was accepted.

The caller is the goroutine that reads the container's log, which is what drives
the run, so this never waits: a store that is slow or failing must not delay the
next case's result or the end of the run. The queue is deep enough for a burst of
cases finishing faster than they can be stored, and a deeper backlog is refused so
the pending area stays bounded.
*/
func (p *outputPublisher) Submit(req publishRequest) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed || p.degraded.Load() {
		return false
	}

	select {
	case p.tasks <- req:
		return true
	default:
		return false
	}
}

// Close stops the workers, waits for the queued uploads and returns how many
// files were published. The pending area of the run is removed afterwards, so a
// finished run leaves nothing behind even when an upload was abandoned.
func (p *outputPublisher) Close() int {
	p.closeOnce.Do(func() {
		p.mu.Lock()
		p.closed = true
		close(p.tasks)
		p.mu.Unlock()

		p.wg.Wait()

		pending := filepath.Join(p.engine.cfg.ExecDir,
			"pending_"+strconv.FormatInt(p.commitID, 10))
		if err := os.RemoveAll(pending); err != nil {
			p.engine.logger.Warn("could not remove the pending upload directory",
				"commit_id", p.commitID, "dir", pending, "error", err,
			)
		}
	})

	return int(p.files.Load())
}
