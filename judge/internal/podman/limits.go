package podman

import (
	spec "github.com/opencontainers/runtime-spec/specs-go"
)

// cpuPeriodMicros is the cgroup accounting window a CPUQuota is expressed
// against, in microseconds (the cgroup v2 convention). A quota equal to the
// period is one full core.
const cpuPeriodMicros = 100_000

// Limits bounds the resources a graded container may consume. It is the
// judge-side backstop for the in-container monitor: the submission runs with the
// same privileges as the monitor and can defeat it, so without cgroup limits a
// fork bomb or a runaway allocation competes with — and can take down — the judge
// process and every other concurrent run.
//
// A zero field disables that particular limit, which is only appropriate where
// the runtime cannot enforce it.
type Limits struct {
	// MemoryBytes caps memory (and, by extension, swap) in bytes.
	MemoryBytes int64
	// PidsLimit caps the number of processes and threads, bounding fork bombs.
	PidsLimit int64
	// CPUQuota is the CPU time budget per CPUPeriodMicros, so 100_000 is one
	// full core and 200_000 is two.
	CPUQuota int64
}

// Configured reports whether any limit is set.
func (l Limits) Configured() bool {
	return l.MemoryBytes > 0 || l.PidsLimit > 0 || l.CPUQuota > 0
}

// Resources renders the limits as a runtime-spec resource block, or nil when no
// limit is configured so the container keeps the runtime default.
func (l Limits) Resources() *spec.LinuxResources {
	if !l.Configured() {
		return nil
	}

	res := &spec.LinuxResources{}

	if l.MemoryBytes > 0 {
		// The swap limit is the memory+swap total, so capping both at the same
		// value leaves the container no swap to escape the memory cap through.
		mem := l.MemoryBytes
		res.Memory = &spec.LinuxMemory{Limit: &mem, Swap: &mem}
	}

	if l.PidsLimit > 0 {
		pids := l.PidsLimit
		res.Pids = &spec.LinuxPids{Limit: &pids}
	}

	if l.CPUQuota > 0 {
		quota := l.CPUQuota
		period := uint64(cpuPeriodMicros)
		res.CPU = &spec.LinuxCPU{Quota: &quota, Period: &period}
	}

	return res
}
