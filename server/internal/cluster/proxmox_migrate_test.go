package cluster

import (
	"context"
	"net/http"
	"slices"
	"testing"
)

const testMigTarget = "node02"

//nolint:gocyclo // single httptest flow covering precheck parsing and the form
func TestProxmox_MigrationPrecheckAndMigrate(t *testing.T) {
	t.Parallel()

	var gotForm map[string][]string

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("GET /api2/json/nodes/node01/qemu/101/migrate", func(w http.ResponseWriter, _ *http.Request) {
			writeJSONFixture(t, w, `{"data":{
				"allowed_nodes":["node03","node02"],
				"not_allowed_nodes":{"node04":{"unavailable_storages":["local-lvm","fast"],"unavailable-resources":["hostpci0"]}},
				"local_disks":[{"volid":"local-lvm:vm-101-disk-0"}],
				"local_resources":["hostpci0"]
			}}`)
		})
		mux.HandleFunc("POST /api2/json/nodes/node01/qemu/101/migrate", func(w http.ResponseWriter, r *http.Request) {
			if err := r.ParseForm(); err != nil {
				t.Fatalf("parse form: %v", err)
			}

			gotForm = r.PostForm

			writeJSONFixture(t, w, `{"data":"UPID:node01:0001:qmigrate:101:pvmss@pve:"}`)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	precheck, err := p.MigrationPrecheck(context.Background(), testNodeName, testVMID)
	if err != nil {
		t.Fatalf("MigrationPrecheck: %v", err)
	}

	wantReason := "unavailable storages: local-lvm, fast; unavailable resources: hostpci0"
	if !slices.Equal(precheck.AllowedNodes, []string{testMigTarget, "node03"}) || precheck.NotAllowed["node04"] != wantReason ||
		!slices.Equal(precheck.LocalDisks, []string{"local-lvm:vm-101-disk-0"}) || !slices.Equal(precheck.LocalResources, []string{"hostpci0"}) {
		t.Fatalf("precheck = %+v", precheck)
	}

	upid, err := p.Migrate(context.Background(), testNodeName, testVMID, MigrateSpec{Target: testMigTarget, Online: true, WithLocalDisks: true})
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	if upid == "" || gotForm["target"][0] != testMigTarget || gotForm["online"][0] != "1" || gotForm["with-local-disks"][0] != "1" {
		t.Fatalf("upid = %q form = %v", upid, gotForm)
	}

	if _, err := p.Migrate(context.Background(), testNodeName, testVMID, MigrateSpec{Target: testMigTarget}); err != nil {
		t.Fatalf("Migrate offline: %v", err)
	}

	if _, hasOnline := gotForm["online"]; hasOnline {
		t.Fatalf("offline form must omit online: %v", gotForm)
	}
}
