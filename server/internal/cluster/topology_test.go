package cluster

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

// Topology must answer from /cluster/resources alone: no per-VM config or
// status read, no version read. Any other path hits the mux 404.
func TestProxmox_TopologySkipsVMHydration(t *testing.T) {
	t.Parallel()

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("GET /api2/json/cluster/resources", func(w http.ResponseWriter, _ *http.Request) {
			writeJSONFixture(t, w, `{"data":[
				{"type":"node","node":"pve1","status":"online"},
				{"type":"qemu","node":"pve1","vmid":100,"status":"running"},
				{"type":"storage","node":"pve1","storage":"local-lvm","status":"available"}
			]}`)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	snap, err := ReadTopology(context.Background(), p)
	if err != nil {
		t.Fatalf("ReadTopology: %v", err)
	}

	if len(snap.Nodes) != 1 || len(snap.VMs) != 1 || len(snap.Storages) != 1 {
		t.Fatalf("snap = %+v, want 1 node, 1 vm, 1 storage", snap)
	}
}

func TestFanOut_KeepsOrderAndReturnsError(t *testing.T) {
	t.Parallel()

	items := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	got, err := fanOut(context.Background(), items, func(_ context.Context, n int) (int, error) { return n * 2, nil })
	if err != nil {
		t.Fatalf("fanOut: %v", err)
	}

	for i, v := range got {
		if v != items[i]*2 {
			t.Fatalf("got[%d] = %d, want %d", i, v, items[i]*2)
		}
	}

	boom := errors.New("boom")

	_, err = fanOut(context.Background(), items, func(_ context.Context, n int) (int, error) {
		if n == 5 {
			return 0, boom
		}

		return n, nil
	})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}
}
