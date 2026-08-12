//nolint:paralleltest,wsl_v5 // test temporarily shortens the shared cascade bound
package pools

import (
	"context"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"testing"
	"time"
)

type timeoutAudit struct{}

func (timeoutAudit) RecordAction(context.Context, string, string, int, string) error { return nil }

type timeoutRefresher struct{}

func (timeoutRefresher) Refresh(context.Context) (time.Time, error) { return time.Time{}, nil }

func TestDelete_BoundedWaitThenProceed(t *testing.T) {
	cluster.ResetFake()
	t.Cleanup(cluster.ResetFake)
	client := cluster.Fake{}
	if err := client.CreatePool(context.Background(), "stuck", ""); err != nil {
		t.Fatalf("CreatePool: %v", err)
	}
	index := inventory.BuildIndex(cluster.Snapshot{VMs: []cluster.VM{{VMID: 1, Name: "legacy", Node: cluster.FakeNode01, Pool: "stuck"}}})
	projection := inventory.NewProjectionFromIndex(&index)
	previous := maxDeleteWait
	maxDeleteWait = time.Millisecond
	t.Cleanup(func() { maxDeleteWait = previous })

	started := time.Now()
	result, err := Delete(context.Background(), auth.Identity{Username: "admin", IsAdmin: true}, client, projection, "default", "stuck", client, timeoutAudit{}, timeoutRefresher{})
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if result.Status != "deleted" {
		t.Fatalf("result = %+v", result)
	}
	if time.Since(started) > time.Second {
		t.Fatalf("bounded delete took too long: %v", time.Since(started))
	}
	pools, err := client.ListPools(context.Background())
	if err != nil {
		t.Fatalf("ListPools: %v", err)
	}
	for _, pool := range pools {
		if pool.Name == "stuck" {
			t.Fatal("pool still exists after bounded wait")
		}
	}
}
