package cluster

import (
	"context"
	"fmt"
	"net/http"
	"testing"
)

// Shared snapshot test fixtures — goconst keeps repeated literals in one place.
const (
	testSnapshotName    = "before-upgrade"
	testDiskFormatQCow2 = "qcow2"
	testDiskFormatRaw   = "raw"
)

func TestProxmox_ListSnapshots_FiltersCurrent(t *testing.T) {
	t.Parallel()

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("GET /api2/json/nodes/node01/qemu/101/snapshot", func(w http.ResponseWriter, _ *http.Request) {
			writeJSONFixture(t, w, fmt.Sprintf(`{"data":[
				{"name":"current"},
				{"name":%q,"description":"pre-upgrade","snaptime":1700000000,"vmstate":1}
			]}`, testSnapshotName))
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	snapshots, err := p.ListSnapshots(context.Background(), testNodeName, testVMID)
	if err != nil {
		t.Fatalf("ListSnapshots: %v", err)
	}

	if len(snapshots) != 1 || snapshots[0].Name != testSnapshotName {
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

	upid, err := p.CreateSnapshot(context.Background(), testNodeName, testVMID, testSnapshotName, "", true)
	if err != nil {
		t.Fatalf("CreateSnapshot: %v", err)
	}

	if upid == "" {
		t.Error("expected a non-empty UPID")
	}

	if gotName != testSnapshotName || gotVMState != "1" {
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
			_, err := p.RollbackSnapshot(ctx, testNodeName, testVMID, testSnapshotName)
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
			_, err := p.DeleteSnapshot(ctx, testNodeName, testVMID, testSnapshotName)
			return err
		},
	})
}

// TestProxmox_DeleteSnapshot_SendsForce — ticket 06: the DELETE carries
// force=1 so an NFS/qcow2 ESTALE cannot leave the VM stuck at
// lock=snapshot-delete (pegaprox incident #422).
func TestProxmox_DeleteSnapshot_SendsForce(t *testing.T) {
	t.Parallel()

	var gotQuery string

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("DELETE /api2/json/nodes/node01/qemu/101/snapshot/before-upgrade", func(w http.ResponseWriter, r *http.Request) {
			gotQuery = r.URL.RawQuery

			writeJSONFixture(t, w, `{"data":"UPID:node01:...:qmdelsnapshot:101:pvmss@pve:"}`)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	if _, err := p.DeleteSnapshot(context.Background(), testNodeName, testVMID, testSnapshotName); err != nil {
		t.Fatalf("DeleteSnapshot: %v", err)
	}

	if gotQuery != "force=1" {
		t.Errorf("query = %q, want force=1", gotQuery)
	}
}

// TestProxmox_SnapshotConfig — ticket 08: a named snapshot reads
// /snapshot/{name}/config; the pseudo-entry "current" reads /config?current=1.
func TestProxmox_SnapshotConfig(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		snapName   string
		wantPath   string
		wantConfig map[string]string
	}{
		{
			name:     "named snapshot",
			snapName: testSnapshotName,
			wantPath: "/api2/json/nodes/node01/qemu/101/snapshot/before-upgrade/config",
			wantConfig: map[string]string{
				"memory": "2048",
				"cores":  "2",
			},
		},
		{
			name:     "current",
			snapName: "current",
			wantPath: "/api2/json/nodes/node01/qemu/101/config?current=1",
			wantConfig: map[string]string{
				"memory": "4096",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var gotPath string

			srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
				mux.HandleFunc("GET /api2/json/nodes/node01/qemu/101/snapshot/before-upgrade/config", func(w http.ResponseWriter, r *http.Request) {
					recordPath(&gotPath, r)
					writeJSONFixture(t, w, `{"data":{"memory":"2048","cores":"2"}}`)
				})
				mux.HandleFunc("GET /api2/json/nodes/node01/qemu/101/config", func(w http.ResponseWriter, r *http.Request) {
					recordPath(&gotPath, r)
					writeJSONFixture(t, w, `{"data":{"memory":"4096"}}`)
				})
			})

			p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

			config, err := p.SnapshotConfig(context.Background(), testNodeName, testVMID, tc.snapName)
			if err != nil {
				t.Fatalf("SnapshotConfig: %v", err)
			}

			if gotPath != tc.wantPath {
				t.Errorf("path = %q, want %q", gotPath, tc.wantPath)
			}

			if config["memory"] != tc.wantConfig["memory"] {
				t.Errorf("memory = %q, want %q", config["memory"], tc.wantConfig["memory"])
			}
		})
	}
}

// recordPath stores the request URL (path + optional query) into dst. Shared
// by the snapshot-config test handlers to keep TestProxmox_SnapshotConfig
// under SonarQube's go:S3776 cognitive-complexity limit.
func recordPath(dst *string, r *http.Request) {
	*dst = r.URL.Path
	if r.URL.RawQuery != "" {
		*dst += "?" + r.URL.RawQuery
	}
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
