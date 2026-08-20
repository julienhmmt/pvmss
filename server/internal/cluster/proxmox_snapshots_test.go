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

	runSnapshotMutationTest(t, snapshotMutationCase{
		method:     http.MethodPost,
		pathSuffix: "/rollback",
		upidKind:   "qmrollback",
		wantPath:   "/api2/json/nodes/node01/qemu/101/snapshot/before-upgrade/rollback",
		call: func(p Proxmox, ctx context.Context) error {
			_, err := p.RollbackSnapshot(ctx, testNodeName, testVMID, "before-upgrade")
			return err
		},
	})
}

func TestProxmox_DeleteSnapshot(t *testing.T) {
	t.Parallel()

	runSnapshotMutationTest(t, snapshotMutationCase{
		method:     http.MethodDelete,
		pathSuffix: "",
		upidKind:   "qmdelsnapshot",
		wantPath:   "/api2/json/nodes/node01/qemu/101/snapshot/before-upgrade",
		call: func(p Proxmox, ctx context.Context) error {
			_, err := p.DeleteSnapshot(ctx, testNodeName, testVMID, "before-upgrade")
			return err
		},
	})
}

type snapshotMutationCase struct {
	method     string
	pathSuffix string
	upidKind   string
	wantPath   string
	call       func(p Proxmox, ctx context.Context) error
}

// runSnapshotMutationTest exercises a snapshot mutation (rollback or delete)
// against a single-shot Proxmox test server. Extracted from
// TestProxmox_RollbackSnapshot and TestProxmox_DeleteSnapshot to satisfy dupl.
func runSnapshotMutationTest(t *testing.T, tc snapshotMutationCase) {
	t.Helper()

	var gotPath string

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc(tc.method+" /api2/json/nodes/node01/qemu/101/snapshot/before-upgrade"+tc.pathSuffix, func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path

			writeJSONFixture(t, w, `{"data":"UPID:node01:...:`+tc.upidKind+`:101:pvmss@pve:"}`)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	if err := tc.call(p, context.Background()); err != nil {
		t.Fatalf("snapshot mutation: %v", err)
	}

	if gotPath != tc.wantPath {
		t.Errorf("path = %q, want %q", gotPath, tc.wantPath)
	}
}
