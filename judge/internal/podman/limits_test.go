package podman

import "testing"

func TestLimitsResourcesDisabled(t *testing.T) {
	if res := (Limits{}).Resources(); res != nil {
		t.Fatalf("empty limits produced %+v, want nil", res)
	}
}

func TestLimitsResourcesCapsMemoryPidsAndCPU(t *testing.T) {
	res := Limits{MemoryBytes: 64 << 20, PidsLimit: 32, CPUQuota: 50_000}.Resources()
	if res == nil {
		t.Fatal("configured limits produced no resource block")
	}

	if res.Memory == nil || res.Memory.Limit == nil || *res.Memory.Limit != 64<<20 {
		t.Fatalf("memory limit = %+v, want 64MiB", res.Memory)
	}
	// Swap is the memory+swap total, so it must be pinned to the same value:
	// otherwise the container swaps past the memory cap.
	if res.Memory.Swap == nil || *res.Memory.Swap != 64<<20 {
		t.Fatalf("swap limit = %+v, want 64MiB", res.Memory.Swap)
	}

	if res.Pids == nil || res.Pids.Limit == nil || *res.Pids.Limit != 32 {
		t.Fatalf("pids limit = %+v, want 32", res.Pids)
	}

	if res.CPU == nil || res.CPU.Quota == nil || *res.CPU.Quota != 50_000 {
		t.Fatalf("cpu quota = %+v, want 50000", res.CPU)
	}
	if res.CPU.Period == nil || *res.CPU.Period != cpuPeriodMicros {
		t.Fatalf("cpu period = %+v, want %d", res.CPU.Period, cpuPeriodMicros)
	}
}

func TestLimitsResourcesLeavesUnsetFieldsAlone(t *testing.T) {
	// Only the PID cap is configured: memory and CPU must stay unlimited rather
	// than being silently set to zero (which the runtime reads as "no limit").
	res := Limits{PidsLimit: 8}.Resources()
	if res == nil {
		t.Fatal("configured limits produced no resource block")
	}

	if res.Memory != nil || res.CPU != nil {
		t.Fatalf("unset limits were populated: memory=%+v cpu=%+v", res.Memory, res.CPU)
	}
}
