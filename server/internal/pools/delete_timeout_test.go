//nolint:paralleltest,wsl_v5 // test temporarily shortens the shared cascade bound
package pools

import (
	"context"
	"errors"
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

// A member still in the pool after the bounded wait must stop the cascade:
// deleting the pool would leave that VM without an owner.
func TestDelete_BoundedWaitRefusesWhenMembersRemain(t *testing.T) {
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
	_, err := Delete(context.Background(), CascadeDeps{Actor: auth.Identity{Username: "admin", IsAdmin: true}, Client: client, Projection: projection, ClusterName: "default", Writer: client, Audit: timeoutAudit{}, Refresher: timeoutRefresher{}}, "stuck")
	if !errors.Is(err, ErrMembersRemain) {
		t.Fatalf("Delete err = %v, want ErrMembersRemain", err)
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
			return
		}
	}
	t.Fatal("pool was deleted while a member remained")
}
