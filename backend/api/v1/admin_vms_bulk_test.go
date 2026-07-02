package apiv1

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
)

func TestValidateBulkVMIDs(t *testing.T) {
	// Build an over-sized slice: maxBulkBatch+1 distinct positive ids.
	tooMany := make([]int, maxBulkBatch+1)
	for i := range tooMany {
		tooMany[i] = i + 1
	}

	tests := []struct {
		name    string
		in      []int
		want    []int
		wantErr bool
	}{
		{name: "happy path", in: []int{101, 102, 103}, want: []int{101, 102, 103}},
		{name: "dedup preserves order", in: []int{102, 101, 102, 101}, want: []int{102, 101}},
		{name: "empty", in: nil, wantErr: true},
		{name: "zero id", in: []int{101, 0}, wantErr: true},
		{name: "negative id", in: []int{-5}, wantErr: true},
		{name: "at cap ok", in: tooMany[:maxBulkBatch], want: tooMany[:maxBulkBatch]},
		{name: "over cap", in: tooMany, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, msg := validateBulkVMIDs(tt.in)
			if tt.wantErr {
				if msg == "" {
					t.Fatalf("expected rejection, got %v", got)
				}
				return
			}
			if msg != "" {
				t.Fatalf("unexpected rejection: %s", msg)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("got %v, want %v", got, tt.want)
				}
			}
		})
	}
}

// TestRunBulkVMAction verifies best-effort fan-out semantics: order-preserving
// results, per-VM success/failure isolation, and "not found" for VMs missing
// from the node map (never dispatched).
func TestRunBulkVMAction(t *testing.T) {
	nodeByVMID := map[int]string{101: "pve1", 102: "pve2", 103: "pve1"}
	vmids := []int{101, 999, 102, 103} // 999 absent → not-found, not dispatched

	var dispatched atomic.Int64
	act := func(_ context.Context, node, vmid, action string) (string, error) {
		dispatched.Add(1)
		if vmid == "102" {
			return "", errors.New("boom")
		}
		return "UPID:" + node + ":" + vmid, nil
	}

	results := runBulkVMAction(context.Background(), "stop", vmids, nodeByVMID, act)

	if len(results) != len(vmids) {
		t.Fatalf("expected %d results, got %d", len(vmids), len(results))
	}
	// Order preserved: results[i] corresponds to vmids[i].
	if results[0].VMID != 101 || !results[0].OK || results[0].TaskID == "" {
		t.Fatalf("result[0] = %+v", results[0])
	}
	if results[1].VMID != 999 || results[1].OK || results[1].Error == "" {
		t.Fatalf("result[1] (missing) = %+v", results[1])
	}
	if results[2].VMID != 102 || results[2].OK || results[2].Error != "proxmox action failed" {
		t.Fatalf("result[2] (failing) = %+v", results[2])
	}
	if results[3].VMID != 103 || !results[3].OK {
		t.Fatalf("result[3] = %+v", results[3])
	}
	// The absent VM must not be dispatched (only 3 real VMs).
	if dispatched.Load() != 3 {
		t.Fatalf("expected 3 dispatches, got %d", dispatched.Load())
	}
}
