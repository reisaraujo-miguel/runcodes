package podman

import (
	"testing"

	spec "github.com/opencontainers/runtime-spec/specs-go"
	"go.podman.io/podman/v6/pkg/specgen"
)

// The container is the only boundary between a submission and the rest of the
// platform, and every part of that boundary is a field on the spec. These tests
// exist so that a refactor cannot quietly drop one of them: they are cheap to
// assert and impossible to notice by reading a diff.

func TestSpecForCutsTheRunOffFromEverything(t *testing.T) {
	s := specFor(RunConfig{
		Name:        "run-1",
		Image:       "ghcr.io/runcodes-icmc/runcodes-runner-c:latest",
		MountSource: "/srv/judge/work/1",
		Labels:      map[string]string{"runcodes.commit": "1"},
	}, Limits{})

	if s.Name != "run-1" {
		t.Errorf("name = %q, want run-1", s.Name)
	}
	if s.Image != "ghcr.io/runcodes-icmc/runcodes-runner-c:latest" {
		t.Errorf("image = %q, want the run's image", s.Image)
	}
	if got := s.Labels["runcodes.commit"]; got != "1" {
		t.Errorf("labels lost the commit: %v", s.Labels)
	}

	// No network namespace at all: a submission that can open a socket reaches
	// the internet and every service on the compose network.
	if s.NetNS.NSMode != specgen.NoNetwork {
		t.Errorf("netns mode = %q, want %q", s.NetNS.NSMode, specgen.NoNetwork)
	}

	for _, ns := range []struct {
		name string
		got  specgen.Namespace
	}{
		{"pidns", s.PidNS},
		{"utsns", s.UtsNS},
		{"ipcns", s.IpcNS},
	} {
		if ns.got.NSMode != specgen.Private {
			t.Errorf("%s mode = %q, want %q", ns.name, ns.got.NSMode, specgen.Private)
		}
	}

	// PR_SET_NO_NEW_PRIVS: without it a setuid binary or a file capability in
	// the image gives the submission privileges the monitor does not have.
	if s.NoNewPrivileges == nil || !*s.NoNewPrivileges {
		t.Errorf("no-new-privileges = %v, want true", s.NoNewPrivileges)
	}

	// Exactly one mount, and it is the workspace: anything else would expose
	// host paths to graded code.
	want := spec.Mount{
		Type:        "bind",
		Source:      "/srv/judge/work/1",
		Destination: "/root",
		Options:     []string{"rw", "z"},
	}
	if len(s.Mounts) != 1 {
		t.Fatalf("mounts = %+v, want exactly the workspace mount", s.Mounts)
	}
	got := s.Mounts[0]
	if got.Type != want.Type || got.Source != want.Source || got.Destination != want.Destination {
		t.Errorf("mount = %+v, want %+v", got, want)
	}
	// "z" is what lets the container's root write to the bind mount on SELinux
	// hosts; without it every run fails there.
	if len(got.Options) != len(want.Options) {
		t.Fatalf("mount options = %v, want %v", got.Options, want.Options)
	}
	for i, opt := range want.Options {
		if got.Options[i] != opt {
			t.Errorf("mount option %d = %q, want %q", i, got.Options[i], opt)
		}
	}
}

func TestSpecForAppliesCgroupLimits(t *testing.T) {
	limits := Limits{MemoryBytes: 32 << 20, PidsLimit: 16, CPUQuota: 25_000}
	s := specFor(RunConfig{Image: "img", MountSource: "/work"}, limits)

	if s.ResourceLimits == nil {
		t.Fatal("configured limits did not reach the spec")
	}
	if s.ResourceLimits.Memory == nil || s.ResourceLimits.Memory.Limit == nil ||
		*s.ResourceLimits.Memory.Limit != 32<<20 {
		t.Errorf("memory limit = %+v, want 32MiB", s.ResourceLimits.Memory)
	}
	if s.ResourceLimits.Pids == nil || s.ResourceLimits.Pids.Limit == nil ||
		*s.ResourceLimits.Pids.Limit != 16 {
		t.Errorf("pids limit = %+v, want 16", s.ResourceLimits.Pids)
	}
}

func TestSpecForLeavesLimitsAloneWhenUnset(t *testing.T) {
	// Limits{} is what a development instance uses; it must mean "runtime
	// default", not "no memory, no processes".
	s := specFor(RunConfig{Image: "img", MountSource: "/work"}, Limits{})

	if s.ResourceLimits != nil {
		t.Errorf("unset limits produced %+v, want nil", s.ResourceLimits)
	}
}
