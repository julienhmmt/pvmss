package httpapi_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

const profileImportPath = "/api/v1/admin/profiles/import"

//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminProfiles_Reorder(t *testing.T) {
	handler, authHandler, st := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)

	rec := adminPut(t, handler, authHandler, cookie, "/api/v1/admin/profiles/order",
		`{"cluster":"default","ids":["large","medium","small"]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}

	var profiles []adminProfileDTO

	list := adminGet(t, handler, authHandler, cookie, "/api/v1/admin/profiles?cluster=default")
	if err := json.Unmarshal(list.Body.Bytes(), &profiles); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(profiles) == 0 || profiles[0].ID != "large" {
		t.Errorf("first profile = %+v, want large", profiles)
	}

	if got := countAuditRows(t, st, "admin.profiles.reorder"); got != 1 {
		t.Errorf("audit rows = %d, want 1", got)
	}

	for _, body := range []string{`not json`, `{"cluster":"nope","ids":["small"]}`} {
		if bad := adminPut(t, handler, authHandler, cookie, "/api/v1/admin/profiles/order", body); bad.Code < 400 {
			t.Errorf("body %q: status = %d, want 4xx", body, bad.Code)
		}
	}
}

//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminProfiles_Import(t *testing.T) {
	handler, authHandler, st := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)

	body := `{"clusters":["default"],"profiles":[` +
		`{"id":"huge","label":"Huge","sockets":1,"cpuCores":8,"memoryMB":16384,"diskGB":200,"bus":"scsi"},` +
		`{"id":"small","label":"Small","sockets":1,"cpuCores":1,"memoryMB":1024,"diskGB":10,"bus":"scsi"},` +
		`{"label":"","cpuCores":0}]}`

	rec := adminPost(t, handler, authHandler, cookie, profileImportPath, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}

	var outcomes []struct {
		ID     string `json:"id"`
		Result string `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &outcomes); err != nil {
		t.Fatalf("decode: %v", err)
	}

	got := map[string]int{}
	for _, o := range outcomes {
		got[o.Result]++
	}

	if got["created"] != 1 || got["skipped"] != 1 || got["invalid"] != 1 {
		t.Errorf("outcomes = %+v, want 1 created, 1 skipped, 1 invalid", outcomes)
	}

	if n := countAuditRows(t, st, "admin.profiles.import"); n != 1 {
		t.Errorf("audit rows = %d, want 1", n)
	}

	for _, bad := range []string{
		`not json`,
		`{"clusters":["default"],"profiles":[]}`,
		`{"clusters":[],"profiles":[{"label":"x"}]}`,
		`{"clusters":["nope"],"profiles":[{"label":"x"}]}`,
	} {
		if r := adminPost(t, handler, authHandler, cookie, profileImportPath, bad); r.Code < 400 {
			t.Errorf("body %q: status = %d, want 4xx", bad, r.Code)
		}
	}
}
