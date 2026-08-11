package cluster

import (
	"context"
	"slices"
	"testing"
)

// TestFakeListBridges_ReturnsSupersetOfApproved verifies the fake's
// ListBridges returns a strict superset of what T06 already approved
// (vmbr0, vmbr1) — the demo needs at least one undiscovered bridge to
// approve (vmbr2).
//
//nolint:paralleltest // serial: shared fake dataset
func TestFakeListBridges_ReturnsSupersetOfApproved(t *testing.T) {
	bridges, err := (Fake{}).ListBridges(context.Background())
	if err != nil {
		t.Fatalf("ListBridges: %v", err)
	}

	approved := []string{FakeBridgeVMbr0, "vmbr1"}
	for _, name := range approved {
		if !slices.ContainsFunc(bridges, func(b Bridge) bool { return b.Name == name }) {
			t.Errorf("approved bridge %q missing from ListBridges result", name)
		}
	}

	// At least one bridge beyond the approved set — the demo target.
	var hasUnapproved bool

	for _, b := range bridges {
		if !slices.Contains(approved, b.Name) {
			hasUnapproved = true
		}
	}

	if !hasUnapproved {
		t.Error("expected at least one unapproved bridge in ListBridges result")
	}
}

// TestFakeListISOs_ReturnsSupersetOfApproved verifies the fake's ListISOs
// returns a strict superset of what T06 already approved (debian-12 and
// ubuntu-24, both on local) — the demo needs at least one undiscovered ISO
// to approve (rocky-9).
//
//nolint:paralleltest // serial: shared fake dataset
func TestFakeListISOs_ReturnsSupersetOfApproved(t *testing.T) {
	isos, err := (Fake{}).ListISOs(context.Background())
	if err != nil {
		t.Fatalf("ListISOs: %v", err)
	}

	approved := []struct{ storage, file string }{
		{FakeStorageLocal, "debian-12-generic-amd64.iso"},
		{FakeStorageLocal, "ubuntu-24.04-server-amd64.iso"},
	}

	for _, want := range approved {
		found := slices.ContainsFunc(isos, func(i ISOImage) bool {
			return i.Storage == want.storage && i.File == want.file
		})
		if !found {
			t.Errorf("approved ISO %s:%s missing from ListISOs result", want.storage, want.file)
		}
	}

	// At least one ISO beyond the approved set — the demo target.
	var hasUnapproved bool

	for _, i := range isos {
		matched := slices.ContainsFunc(approved, func(w struct{ storage, file string }) bool {
			return i.Storage == w.storage && i.File == w.file
		})
		if !matched {
			hasUnapproved = true
		}
	}

	if !hasUnapproved {
		t.Error("expected at least one unapproved ISO in ListISOs result")
	}
}
