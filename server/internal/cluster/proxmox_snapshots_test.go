package cluster

import (
	"context"
	"net/http"
	"testing"
)

func TestProxmox_ListSnapshots_FiltersCurrent(t *testing.T) {
	t.Parallel()

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("GET /api2/json/nodes/node01/qemu/101/snapshot", func(w http.ResponseWriter, _ *http.Request) {
			writeJSONFixture(t, w, `{"data":[
				{"name":"current"},
				{"name":"before-upgrade","description":"pre-upgrade","snaptime":1700000000,"vmstate":1}
			]}`)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	snapshots, err := p.ListSnapshots(context.Background(), testNodeName, testVMID)
	if err != nil {
		t.Fatalf("ListSnapshots: %v", err)
	}

	if len(snapshots) != 1 || snapshots[0].Name != "before-upgrade" {
		t.Fatalf("snapshots = %+v, want exactly one (current excluded)", snapshots)
	}

	if !snapshots[0].VMState || snapshots[0].Description != "pre-upgrade" {
		t.Errorf("snapshot = %+v", snapshots[0])
	}
}

func TestProxmox_CreateSnapshot(t *testing.T) {
	t.Parallel()

	var gotName, gotVMState string

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("POST /api2/json/nodes/node01/qemu/101/snapshot", func(w http.ResponseWriter, r *http.Request) {
			if err := r.ParseForm(); err != nil {
				t.Fatalf("parse form: %v", err)
			}

			gotName = r.FormValue("snapname")
			gotVMState = r.FormValue("vmstate")

			writeJSONFixture(t, w, `{"data":"UPID:node01:...:qmsnapshot:101:pvmss@pve:"}`)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	upid, err := p.CreateSnapshot(context.Background(), testNodeName, testVMID, "before-upgrade", "", true)
	if err != nil {
		t.Fatalf("CreateSnapshot: %v", err)
	}

	if upid == "" {
		t.Error("expected a non-empty UPID")
	}

	if gotName != "before-upgrade" || gotVMState != "1" {
		t.Errorf("name=%q vmstate=%q", gotName, gotVMState)
	}
}

func TestProxmox_RollbackSnapshot(t *testing.T) {
	t.Parallel()

	var gotPath string

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("POST /api2/json/nodes/node01/qemu/101/snapshot/before-upgrade/rollback", func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path

			writeJSONFixture(t, w, `{"data":"UPID:node01:...:qmrollback:101:pvmss@pve:"}`)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	if _, err := p.RollbackSnapshot(context.Background(), testNodeName, testVMID, "before-upgrade"); err != nil {
		t.Fatalf("RollbackSnapshot: %v", err)
	}

	if gotPath != "/api2/json/nodes/node01/qemu/101/snapshot/before-upgrade/rollback" {
		t.Errorf("path = %q", gotPath)
	}
}

func TestProxmox_DeleteSnapshot(t *testing.T) {
	t.Parallel()

	var gotPath string

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("DELETE /api2/json/nodes/node01/qemu/101/snapshot/before-upgrade", func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path

			writeJSONFixture(t, w, `{"data":"UPID:node01:...:qmdelsnapshot:101:pvmss@pve:"}`)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	if _, err := p.DeleteSnapshot(context.Background(), testNodeName, testVMID, "before-upgrade"); err != nil {
		t.Fatalf("DeleteSnapshot: %v", err)
	}

	if gotPath != "/api2/json/nodes/node01/qemu/101/snapshot/before-upgrade" {
		t.Errorf("path = %q", gotPath)
	}
}
