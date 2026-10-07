package httpapi

import (
	"pvmss/server/internal/vm"
	"testing"
)

// The projection lags /cluster/resources by seconds after a write; a write
// response must show what was just written, not the stale read.
func TestWrittenValuesOverlayStaleEntity(t *testing.T) {
	t.Parallel()

	stale := vm.Entity{Name: "old", Description: "d", Sockets: 1, Cores: 1, CPUCores: 1, MemoryTotal: 1 << 30}

	renamed := withPatch(stale, patchRequest{Name: "new"})
	if renamed.Name != "new" || renamed.Description != "d" {
		t.Errorf("patch overlay = %q/%q, want new/d", renamed.Name, renamed.Description)
	}

	two, mem := 2, 1536

	resized := withHardware(stale, hardwareRequest{Cores: &two, MemoryMB: &mem})
	if resized.Cores != 2 || resized.CPUCores != 2 || resized.MemoryTotal != 1536<<20 || resized.Sockets != 1 {
		t.Errorf("hardware overlay = %+v", resized)
	}

	if stale.Name != "old" || stale.Cores != 1 {
		t.Error("overlay mutated its input")
	}
}
