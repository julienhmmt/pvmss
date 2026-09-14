package vm_test

import (
	"context"
	"errors"
	"path/filepath"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/config"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/store"
	"pvmss/server/internal/vm"
	"testing"
)

// consolePasswordIndex builds a fresh fake index for console-password tests.
func consolePasswordIndex(t *testing.T) *inventory.Index {
	t.Helper()
	cluster.ResetFake()
	t.Cleanup(cluster.ResetFake)

	snapshot, err := (cluster.Fake{}).Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	index := inventory.BuildIndex(snapshot)

	return &index
}

// consolePasswordStore opens a fresh temp store for console-password tests.
func consolePasswordStore(t *testing.T) *store.Store {
	t.Helper()

	st, err := store.Open(config.Configuration{
		DBPath:    filepath.Join(t.TempDir(), "console-password.db"),
		LogLevel:  testLogLevel,
		LogFormat: testLogFormat,
		LogOutput: testLogOutput,
	})
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}

	t.Cleanup(func() { _ = st.Close() })

	return st
}

// TestSetConsolePassword_GeneratesAndApplies — the action generates a
// random password, applies it via the guest agent to the VM's ciuser, and
// returns it once (issue 05).
//
//nolint:paralleltest // serial: shared fake dataset
func TestSetConsolePassword_GeneratesAndApplies(t *testing.T) {
	index := consolePasswordIndex(t)
	st := consolePasswordStore(t)

	// VM 101 is stopped; start it so the agent path can succeed.
	if err := (cluster.Fake{}).Action(context.Background(), cluster.FakeNode01, 101, "start"); err != nil {
		t.Fatalf("start VM 101: %v", err)
	}

	password, err := vm.SetConsolePassword(context.Background(), vm.ConsolePasswordDeps{
		Index: index, Actor: cloudAliceIdentity(), ClusterName: testClusterName, VMID: 101,
		Reader: cluster.Fake{}, Writer: cluster.Fake{}, Audit: st, Refresher: testRefresher{},
		StatusReader: cluster.Fake{},
	})
	if err != nil {
		t.Fatalf("SetConsolePassword: %v", err)
	}

	if password == "" {
		t.Fatal("password is empty")
	}

	// The password was applied via the guest agent, not cipassword.
	sawAgent := false

	for _, c := range cluster.FakeCallsFor(101) {
		if c.Action == testActionSetCloudInitPassword {
			sawAgent = true
		}
	}

	if !sawAgent {
		t.Fatal("expected the password to be applied via the guest agent (set_cloudinit_password)")
	}
}

// TestSetConsolePassword_RefusesStoppedVM — a stopped VM is refused before
// the agent is probed (the pre-flight check reads the live status).
//
//nolint:paralleltest // serial: shared fake dataset
func TestSetConsolePassword_RefusesStoppedVM(t *testing.T) {
	index := consolePasswordIndex(t)
	st := consolePasswordStore(t)

	// VM 101 is stopped in the pristine dataset — do not start it.
	_, err := vm.SetConsolePassword(context.Background(), vm.ConsolePasswordDeps{
		Index: index, Actor: cloudAliceIdentity(), ClusterName: testClusterName, VMID: 101,
		Reader: cluster.Fake{}, Writer: cluster.Fake{}, Audit: st, Refresher: testRefresher{},
		StatusReader: cluster.Fake{},
	})
	if !errors.Is(err, vm.ErrVMNotRunning) {
		t.Fatalf("error = %v, want ErrVMNotRunning", err)
	}
}
