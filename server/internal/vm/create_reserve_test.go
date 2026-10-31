package vm_test

import (
	"context"
	"errors"
	"log/slog"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/policy"
	"pvmss/server/internal/vm"
	"testing"
)

// The projection only learns about a new VM after the create task and a
// refresh. Until then an in-flight create must still count toward the quota
// and reserve its name, or two quick requests both pass the checks.
//
//nolint:paralleltest // Uses the process-wide cluster.FakeCalls log.
func TestCreate_InFlightCreateCountsBeforeRefresh(t *testing.T) {
	fixture := newCreateFixture(t)
	ctx := context.Background()

	snapshot, err := fixture.fake.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	index := inventory.BuildIndex(snapshot)
	service := policy.New(fixture.store, inventory.NewProjectionFromIndex(&index), fixture.fake)
	used := len(index.ByPool[aliceIdentity().Pool])

	if err := service.SetPolicy(ctx, testClusterName, policy.DefaultGabarit(), used+2); err != nil {
		t.Fatalf("SetPolicy: %v", err)
	}

	deps := vm.CreateDeps{Store: fixture.store, Creator: fixture.fake, Audit: fixture.store, Log: slog.New(slog.DiscardHandler), Services: []*policy.Policy{service}}
	create := func(name string) error {
		req := detailedRequest()
		req.Name = name
		_, err := vm.Create(ctx, aliceIdentity(), req.Cluster, req, deps)

		return err
	}

	if err := create("inflight-a"); err != nil {
		t.Fatalf("first create: %v", err)
	}

	if err := create("inflight-a"); !errors.Is(err, vm.ErrNameTaken) {
		t.Fatalf("same name while in flight: err = %v, want ErrNameTaken", err)
	}

	if err := create("inflight-b"); err != nil {
		t.Fatalf("second create within quota: %v", err)
	}

	if err := create("inflight-c"); !errors.Is(err, policy.ErrQuotaExceeded) {
		t.Fatalf("third create over quota: err = %v, want ErrQuotaExceeded", err)
	}
}

// A projection that is wired but not loaded yet must refuse, not count 0.
//
//nolint:paralleltest // Uses the process-wide cluster.FakeCalls log.
func TestCreate_RefusesBeforeFirstInventoryLoad(t *testing.T) {
	fixture := newCreateFixture(t)
	service := policy.New(fixture.store, inventory.NewProjection(), fixture.fake)

	req := detailedRequest()
	req.Name = "too-early"

	_, err := vm.Create(context.Background(), aliceIdentity(), req.Cluster, req, vm.CreateDeps{Store: fixture.store, Creator: fixture.fake, Audit: fixture.store, Log: slog.New(slog.DiscardHandler), Services: []*policy.Policy{service}})
	if !errors.Is(err, policy.ErrInventoryNotReady) {
		t.Fatalf("Create before first load: err = %v, want ErrInventoryNotReady", err)
	}
}
