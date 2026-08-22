package vm_test

import (
	"context"
	"errors"
	"testing"

	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/vm"
)

// fakeSerialWriter embeds cluster.Fake so EnableSerialConsole's refresh of the
// inventory is exercised against a real dataset, and records whether
// EnableSerial was called.
type fakeSerialWriter struct {
	cluster.Fake
	called bool
}

func (w *fakeSerialWriter) EnableSerial(_ context.Context, node string, vmid int) error {
	w.called = true
	return w.Fake.EnableSerial(context.Background(), node, vmid)
}

// TestEnableSerialConsole_ResolveThenWriterThenAudit — the happy path calls
// Resolve (ownership gate), then Writer.EnableSerial, then records the
// "serial_enable" audit action and refreshes the inventory so HasSerial flips.
//
//nolint:paralleltest // serial: shared fake VM fixture
func TestEnableSerialConsole_ResolveThenWriterThenAudit(t *testing.T) {
	idx := buildResolveIndex(t)
	alice := auth.Identity{Username: cluster.FakeUserAlice, Pool: cluster.FakePoolAlice}
	writer := &fakeSerialWriter{}
	audit := &fakeAuditRecorder{}

	err := vm.EnableSerialConsole(context.Background(), vm.EnableSerialDependencies{
		Index:       idx,
		Actor:       alice,
		ClusterName: testClusterName,
		VMID:        101, // VM 101 starts without a serial port in the dataset
		Writer:      writer,
		Audit:       audit,
		Refresher:   noopRefresher{},
	})
	if err != nil {
		t.Fatalf("EnableSerialConsole: %v", err)
	}

	if !writer.called {
		t.Fatalf("Writer.EnableSerial was not called")
	}

	if audit.gotAction != "serial_enable" || audit.gotVMID != 101 {
		t.Fatalf("audit recorded %+v, want serial_enable/101", audit)
	}

	// The writer-level call is the unit contract here; the refreshed entity's
	// HasSerial flip is covered end-to-end by the httpapi integration test
	// (which drives a real inventory worker that rebuilds from the fake).
	calls := cluster.FakeCallsFor(101)
	if len(calls) != 1 || calls[0].Action != "enable_serial" {
		t.Fatalf("fake calls = %+v, want one enable_serial call", calls)
	}
}

// TestEnableSerialConsole_NonOwnerForbidden — Resolve() gates the write; a
// non-owner gets ErrForbidden before the writer is touched.
//
//nolint:paralleltest // serial: shared fake VM fixture
func TestEnableSerialConsole_NonOwnerForbidden(t *testing.T) {
	idx := buildResolveIndex(t)
	bob := auth.Identity{Username: "bob@pve", Pool: cluster.FakePoolBob}
	writer := &fakeSerialWriter{}
	audit := &fakeAuditRecorder{}

	err := vm.EnableSerialConsole(context.Background(), vm.EnableSerialDependencies{
		Index:       idx,
		Actor:       bob,
		ClusterName: testClusterName,
		VMID:        100, // in pool-alice, not bob's
		Writer:      writer,
		Audit:       audit,
		Refresher:   noopRefresher{},
	})
	if !errors.Is(err, vm.ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}

	if writer.called {
		t.Fatalf("Writer.EnableSerial was called despite Resolve failure")
	}
}
