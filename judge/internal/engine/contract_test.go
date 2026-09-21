package engine

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/runcodes-icmc/judge/internal/config"
	"github.com/runcodes-icmc/judge/internal/model"
)

// harnessPath is the image-side script the judge's config file is written for.
const harnessPath = "../../../runners/base/base-script.sh"

/*
TestHarnessReadsTheJudgeConfig is the cross-component check for the container
contract: the keys writeContainerConfig emits are the keys the image's harness
reads, with the meaning both sides assume.

Both halves are tested on their own elsewhere (the judge's writer in Go, the
harness in runners/test/harness.test.sh), but a rename on one side would
pass both of those tests and only break in production. Here the real base script
runs against a config the judge generated, with a stand-in monitor that records
the arguments it was handed.

It skips when the images' sources are not checked out next to the judge module,
which is the case for a standalone judge checkout.
*/
func TestHarnessReadsTheJudgeConfig(t *testing.T) {
	if _, err := os.Stat(harnessPath); err != nil {
		t.Skipf("the image sources are not present: %v", err)
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash is not available")
	}

	dir := t.TempDir()

	// The stand-in monitor: it records its arguments, creates the files the real
	// monitor's -o/-e would create, writes the report at -r and exits cleanly.
	monitorLog := filepath.Join(dir, "monitor-args")
	monitor := filepath.Join(dir, "fake-monitor")
	script := `#!/bin/bash
printf '%s\n' "$*" >>"` + monitorLog + `"
while [ "$#" -gt 0 ]; do
    case "$1" in
        -r) printf 'exit_status=0\nsignal=\ntime=0.01\n' >"$2" ;;
        -o | -e) : >"$2" ;;
    esac
    shift
done
`
	if err := os.WriteFile(monitor, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	// A workspace shaped like the judge's, including the harness's own monitor
	// override, which the images bake in as /usr/bin/monitor.
	ws := filepath.Join(dir, "ws")
	for _, d := range []string{"src", "test_1"} {
		if err := os.MkdirAll(filepath.Join(ws, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(ws, "src", "prog.sh"), []byte("#!/bin/bash\necho ok\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "1.in"), []byte("in\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	engine := &Engine{cfg: &config.Config{
		ExecDir:            dir,
		MonitorMaxFileSize: 5 * 1024 * 1024,
		MonitorMaxMemSize:  256 * 1024 * 1024,
		CompilationTimeout: 10 * time.Second,
		DefaultCaseTimeout: 3 * time.Second,
	}}

	workspace := &workspace{
		BaseDir:    ws,
		RunNonce:   "contract-nonce",
		Language:   nil, // no compilation phase for this script
		Compilable: false,
		TestCases: []model.TestCase{{
			ID:            1,
			CPUTimeLimit:  7,
			MemUsageLimit: 1 << 20,
			FileSizeLimit: 1 << 21,
			StackLimit:    1 << 19,
		}},
	}

	if err := engine.writeContainerConfig(&model.Commit{ID: 1, S3Key: "uuid/prog.sh"}, workspace); err != nil {
		t.Fatalf("writeContainerConfig: %v", err)
	}

	// The images bake in /usr/bin/monitor and the judge never sends that key, so
	// the test points the harness at its stand-in here.
	configPath := filepath.Join(ws, "container.config")
	configFile, err := os.OpenFile(configPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := configFile.WriteString("monitor_bin=" + monitor + "\n"); err != nil {
		t.Fatal(err)
	}
	if err := configFile.Close(); err != nil {
		t.Fatal(err)
	}

	// The image is the base script plus a language script, as the Dockerfiles
	// build it. This one runs the submitted script and nothing else.
	image := filepath.Join(dir, "runcodes")
	base, err := os.ReadFile(harnessPath)
	if err != nil {
		t.Fatal(err)
	}
	lang := `
run_command="bash ${src_file}"
run_tests "${run_command}"
`
	if err := os.WriteFile(image, append(base, []byte(lang)...), 0o755); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(bash, image)
	cmd.Dir = ws
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("the harness failed: %v\n%s", err, output)
	}
	log := string(output)

	// The nonce the judge generated is what the harness prints with a milestone,
	// which is what the judge's awaitLine matches on.
	for _, want := range []string{
		"run.start contract-nonce",
		"run.done contract-nonce",
	} {
		if !strings.Contains(log, want) {
			t.Errorf("missing %q in the harness output:\n%s", want, log)
		}
	}

	// The per-case limits the judge wrote reached the monitor as its arguments.
	args, err := os.ReadFile(monitorLog)
	if err != nil {
		t.Fatalf("the monitor was never invoked: %v", err)
	}
	for _, want := range []string{
		"-m 1048576",   // ms_1
		"-f 2097152",   // fs_1
		"-s 524288",    // stack_1
		"-i ../1.in",   // the case input
		"bash prog.sh", // the command under test
	} {
		if !strings.Contains(string(args), want) {
			t.Errorf("the harness did not pass %q to the monitor; it passed:\n%s", want, args)
		}
	}
}
