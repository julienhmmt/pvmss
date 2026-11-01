package policy_test

import (
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"testing"
)

// StorageFreeBytes answers the soft free-space check from the projection:
// Total-Used when the storage is indexed, zero (not an error) when the
// projection or the storage is absent - the live check is the caller's job.
func TestStorageFreeBytes(t *testing.T) {
	t.Parallel()

	index := &inventory.Index{
		StoragesByNode: map[string][]cluster.Storage{
			"n1": {
				{Name: "local-lvm", Total: 100, Used: 40},
				{Name: "shared-nfs", Total: 200, Used: 200},
			},
		},
	}
	service := newPolicyServiceWithProjectionOnly(t, inventory.NewProjectionFromIndex(index))
	cases := []struct {
		name    string
		node    string
		storage string
		want    int64
	}{
		{name: "known storage", node: "n1", storage: "local-lvm", want: 60},
		{name: "full storage is zero not negative", node: "n1", storage: "shared-nfs", want: 0},
		{name: "unknown storage", node: "n1", storage: "ghost", want: 0},
		{name: "unknown node", node: "ghost", storage: "local-lvm", want: 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := service.StorageFreeBytes("default", tc.node, tc.storage)
			if err != nil {
				t.Fatalf("StorageFreeBytes: %v", err)
			}

			if got != tc.want {
				t.Errorf("StorageFreeBytes = %d, want %d", got, tc.want)
			}
		})
	}
}
