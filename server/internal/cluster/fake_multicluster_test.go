//nolint:wsl_v5 // table-driven client contract keeps calls together
package cluster_test

import (
	"context"
	"errors"
	"pvmss/server/internal/cluster"
	"testing"
)

//nolint:paralleltest // fake contract tests share package fixture state
func TestFake_OfflineDemoRejectsEveryClientMethod(t *testing.T) {
	fake := cluster.Fake{ClusterName: "offline-demo"}
	ctx := context.Background()
	tests := []struct {
		name string
		call func() error
	}{
		{name: "snapshot", call: func() error { _, err := fake.Snapshot(ctx); return err }},
		{name: "authenticate", call: func() error { _, err := fake.Authenticate(ctx, "alice", "password"); return err }},
		{name: "change password", call: func() error { return fake.ChangePassword(ctx, "alice", "old", "new") }},
		{name: "bridges", call: func() error { _, err := fake.ListBridges(ctx); return err }},
		{name: "isos", call: func() error { _, err := fake.ListISOs(ctx); return err }},
		{name: "pools", call: func() error { _, err := fake.ListPools(ctx); return err }},
		{name: "ensure role", call: func() error { return fake.EnsurePoolRole(ctx) }},
		{name: "ensure user", call: func() error { _, err := fake.EnsurePoolUser(ctx, "pool", "password"); return err }},
		{name: "create pool", call: func() error { return fake.CreatePool(ctx, "pool", "comment") }},
		{name: "set ACL", call: func() error { return fake.SetPoolACL(ctx, "user", "pool", "role") }},
		{name: "delete pool", call: func() error { return fake.DeletePool(ctx, "pool") }},
		{name: "delete user", call: func() error { return fake.DeleteUser(ctx, "user") }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if err := test.call(); !errors.Is(err, cluster.ErrUnreachable) {
				t.Fatalf("error = %v, want ErrUnreachable", err)
			}
		})
	}
}
