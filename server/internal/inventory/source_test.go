package inventory_test

import (
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"testing"
)

func TestIndex_All_ReturnsMapWithEmptyKey(t *testing.T) {
	t.Parallel()

	idx := inventory.BuildIndex(fakeSnapshot())

	all := idx.All()
	if len(all) != 1 {
		t.Fatalf("len(All) = %d, want 1", len(all))
	}

	got, ok := all[""]
	if !ok {
		t.Fatal(`missing "" key in All() result`)
	}

	if got != &idx {
		t.Fatal("All()[\"\"] does not point to the same index")
	}
}

func TestIndex_Lookup_FoundAndNotFound(t *testing.T) {
	t.Parallel()

	idx := inventory.BuildIndex(fakeSnapshot())

	cases := []struct {
		name   string
		vmid   int
		wantOK bool
		wantVM cluster.VM
	}{
		{"found vmid 100", 100, true, idx.ByVMID[100]},
		{"found vmid 101", 101, true, idx.ByVMID[101]},
		{"not found vmid 999", 999, false, cluster.VM{}},
		{"not found vmid 0", 0, false, cluster.VM{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, ok := idx.Lookup("", tc.vmid)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}

			if tc.wantOK {
				if got.VMID != tc.wantVM.VMID {
					t.Errorf("VMID = %d, want %d", got.VMID, tc.wantVM.VMID)
				}
				if got.Name != tc.wantVM.Name {
					t.Errorf("Name = %q, want %q", got.Name, tc.wantVM.Name)
				}
			}
		})
	}
}

func TestIndex_Lookup_NilIndexReturnsFalse(t *testing.T) {
	t.Parallel()

	var idx *inventory.Index

	got, ok := idx.Lookup("anything", 100)
	if ok {
		t.Fatal("ok = true, want false for nil index")
	}

	if got.VMID != 0 {
		t.Fatalf("VMID = %d, want 0 for nil index", got.VMID)
	}
}

func TestProjection_All_ReturnsMapWithEmptyKey(t *testing.T) {
	t.Parallel()

	idx := inventory.BuildIndex(fakeSnapshot())
	projection := inventory.NewProjectionFromIndex(&idx)

	all := projection.All()
	if len(all) != 1 {
		t.Fatalf("len(All) = %d, want 1", len(all))
	}

	got, ok := all[""]
	if !ok {
		t.Fatal(`missing "" key in Projection.All() result`)
	}

	if got == nil {
		t.Fatal("All()[\"\"] is nil")
	}

	if got.ByVMID[100].VMID != 100 {
		t.Errorf("All()[\"\"] does not contain vmid 100")
	}
}

func TestProjection_All_EmptyProjectionReturnsNilIndex(t *testing.T) {
	t.Parallel()

	projection := inventory.NewProjection()

	all := projection.All()
	if len(all) != 1 {
		t.Fatalf("len(All) = %d, want 1", len(all))
	}

	got, ok := all[""]
	if !ok {
		t.Fatal(`missing "" key in Projection.All() result`)
	}

	if got != nil {
		t.Fatalf("All()[\"\"] = %v, want nil for empty projection", got)
	}
}

func TestRegistry_Lookup_FoundAndNotFound(t *testing.T) {
	t.Parallel()

	idx := inventory.BuildIndex(fakeSnapshot())
	registry := inventory.NewRegistryFromIndexes(map[string]*inventory.Index{"default": &idx})

	cases := []struct {
		name        string
		clusterName string
		vmid        int
		wantOK      bool
	}{
		{"found default vmid 100", "default", 100, true},
		{"not found default vmid 999", "default", 999, false},
		{"unknown cluster", "nonexistent", 100, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, ok := registry.Lookup(tc.clusterName, tc.vmid)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}

			if tc.wantOK && got.VMID != tc.vmid {
				t.Errorf("VMID = %d, want %d", got.VMID, tc.vmid)
			}
		})
	}
}

func TestRegistry_All_ReturnsClusterIndexes(t *testing.T) {
	t.Parallel()

	idx := inventory.BuildIndex(fakeSnapshot())
	registry := inventory.NewRegistryFromIndexes(map[string]*inventory.Index{"default": &idx})

	all := registry.All()
	if len(all) != 1 {
		t.Fatalf("len(All) = %d, want 1", len(all))
	}

	got, ok := all["default"]
	if !ok {
		t.Fatal(`missing "default" key in Registry.All() result`)
	}

	if got == nil {
		t.Fatal("All()[\"default\"] is nil")
	}

	if got.ByVMID[100].VMID != 100 {
		t.Errorf("All()[\"default\"] does not contain vmid 100")
	}
}
