package cluster

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"testing"
)

func TestProxmox_SetBootOrder(t *testing.T) {
	t.Parallel()

	var gotBoot string

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("PUT /api2/json/nodes/node01/qemu/101/config", func(w http.ResponseWriter, r *http.Request) {
			if err := r.ParseForm(); err != nil {
				t.Fatalf("parse form: %v", err)
			}

			gotBoot = r.FormValue("boot")

			writeJSONFixture(t, w, `{"data":null}`)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	if err := p.SetBootOrder(context.Background(), testNodeName, testVMID, []string{cdromDiskKey, diskKeySCSI0}); err != nil {
		t.Fatalf("SetBootOrder: %v", err)
	}

	if gotBoot != "order="+cdromDiskKey+";"+diskKeySCSI0 {
		t.Errorf("boot = %q, want order=%s;%s", gotBoot, cdromDiskKey, diskKeySCSI0)
	}
}

func TestProxmox_SetBootOrder_EmptyDeletes(t *testing.T) {
	t.Parallel()

	var gotDelete string

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("PUT /api2/json/nodes/node01/qemu/101/config", func(w http.ResponseWriter, r *http.Request) {
			if err := r.ParseForm(); err != nil {
				t.Fatalf("parse form: %v", err)
			}

			gotDelete = r.FormValue("delete")

			writeJSONFixture(t, w, `{"data":null}`)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	if err := p.SetBootOrder(context.Background(), testNodeName, testVMID, nil); err != nil {
		t.Fatalf("SetBootOrder: %v", err)
	}

	if gotDelete != "boot" {
		t.Errorf("delete = %q, want boot", gotDelete)
	}
}

func TestFake_SetBootOrder(t *testing.T) {
	t.Parallel()

	fake := NewFake("test-boot-order")
	ctx := context.Background()

	if err := fake.SetBootOrder(ctx, FakeNode01, 100, []string{cdromDiskKey, diskKeySCSI0}); err != nil {
		t.Fatalf("SetBootOrder: %v", err)
	}

	snap, err := fake.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	for _, vm := range snap.VMs {
		if vm.VMID == 100 && !slices.Equal(vm.BootOrder, []string{cdromDiskKey, diskKeySCSI0}) {
			t.Fatalf("BootOrder = %v, want [ide2 scsi0]", vm.BootOrder)
		}
	}

	// An empty order clears the explicit boot config.
	if err := fake.SetBootOrder(ctx, FakeNode01, 100, nil); err != nil {
		t.Fatalf("SetBootOrder(empty): %v", err)
	}

	snap, err = fake.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	for _, vm := range snap.VMs {
		if vm.VMID == 100 && len(vm.BootOrder) != 0 {
			t.Fatalf("BootOrder = %v, want empty", vm.BootOrder)
		}
	}
}

func TestFake_SetBootOrder_NotFound(t *testing.T) {
	t.Parallel()

	if err := (Fake{}).SetBootOrder(context.Background(), FakeNode01, 99999, []string{"ide2"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("SetBootOrder(not found) error = %v, want ErrNotFound", err)
	}
}
