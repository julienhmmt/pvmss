package httpapi_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

type configFieldDTO struct {
	Name     string  `json:"name"`
	Value    *string `json:"value"`
	Redacted bool    `json:"redacted"`
}

type clusterHealthDTO struct {
	Name                 string `json:"name"`
	RefreshedAt          string `json:"refreshedAt"`
	LastRefreshSucceeded bool   `json:"lastRefreshSucceeded"`
}

type appInfoDTO struct {
	Version  string             `json:"version"`
	Config   []configFieldDTO   `json:"config"`
	Clusters []clusterHealthDTO `json:"clusters"`
}

type publicVersionDTO struct {
	Version string `json:"version"`
}

// TestAdminAppInfo_AsAdmin_ReturnsVersionAndConfig — T041: GET /admin/appinfo
// as admin returns version, config fields with AdminPasswordHash redacted,
// and per-cluster health.
//

func TestAdminAppInfo_AsAdmin_ReturnsVersionAndConfig(t *testing.T) {
	// Set a known admin password hash in the env so config.Load succeeds and
	// the redaction test is meaningful. t.Setenv handles cleanup automatically.
	t.Setenv("ADMIN_PASSWORD_HASH", "$2a$10$knownhashvaluefor test only")
	t.Setenv("SESSION_SECRET", strings.Repeat("s", 32))

	ops, auth, _ := newAdminOpsHandler(t)
	cookie := adminCookie(t, auth)

	rec := opsGet(t, ops, auth, cookie, "/api/v1/admin/appinfo")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}

	var info appInfoDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &info); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if info.Version == "" {
		t.Error("version is empty")
	}

	// Find the ADMIN_PASSWORD_HASH field — must be redacted with null value.
	for _, f := range info.Config {
		if f.Name == "ADMIN_PASSWORD_HASH" {
			if !f.Redacted {
				t.Error("ADMIN_PASSWORD_HASH redacted = false, want true")
			}

			if f.Value != nil {
				t.Errorf("ADMIN_PASSWORD_HASH value = %v, want nil (JSON null)", *f.Value)
			}
		}
	}

	// Per-cluster health from the projection.
	if len(info.Clusters) == 0 {
		t.Error("clusters is empty — projection health not read")
	}
}

// TestAdminAppInfo_Sc006_HashNotInResponseBody — T042/SC-006: the full
// response body of GET /admin/appinfo does not contain the configured admin
// password hash as a substring.
//

func TestAdminAppInfo_Sc006_HashNotInResponseBody(t *testing.T) {
	hash := "$2a$10$N9qo8uLOickgx2ZMRZoMy.MrqK0aVzJqDOKU3FqZwM9FqZwM9FqZwM9"
	t.Setenv("ADMIN_PASSWORD_HASH", hash)
	t.Setenv("SESSION_SECRET", strings.Repeat("s", 32))

	ops, auth, _ := newAdminOpsHandler(t)
	cookie := adminCookie(t, auth)

	rec := opsGet(t, ops, auth, cookie, "/api/v1/admin/appinfo")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}

	if strings.Contains(rec.Body.String(), hash) {
		t.Fatal("admin password hash appears in the appinfo response body — SC-006 violation")
	}

	// Also verify the JSON structure: ADMIN_PASSWORD_HASH reports
	// {"redacted": true, "value": null}.
	var info appInfoDTO

	_ = json.Unmarshal(rec.Body.Bytes(), &info)

	for _, f := range info.Config {
		if f.Name == "ADMIN_PASSWORD_HASH" {
			if !f.Redacted || f.Value != nil {
				t.Errorf("ADMIN_PASSWORD_HASH = %+v, want redacted=true value=nil", f)
			}
		}
	}
}

// TestAdminAppInfo_AsNonAdmin_Returns403 — T043: GET /admin/appinfo as
// non-admin returns 403; GET /public/version as non-admin and as no identity
// returns 200 both times.
//
//nolint:paralleltest // serial: shared env
func TestAdminAppInfo_AsNonAdmin_Returns403(t *testing.T) {
	ops, auth, _ := newAdminOpsHandler(t)
	aliceCookie := loginCookie(t, auth, `{"username":"alice","password":"pvmss-alice"}`)

	rec := opsGet(t, ops, auth, aliceCookie, "/api/v1/admin/appinfo")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("appinfo as non-admin = %d, want 403", rec.Code)
	}

	// Public version succeeds for non-admin.
	pubRec := opsGet(t, ops, auth, aliceCookie, "/api/v1/public/version")
	if pubRec.Code != http.StatusOK {
		t.Errorf("public version as non-admin = %d, want 200", pubRec.Code)
	}

	// Public version succeeds with no identity at all.
	noAuthRec := opsGet(t, ops, auth, nil, "/api/v1/public/version")
	if noAuthRec.Code != http.StatusOK {
		t.Errorf("public version as no identity = %d, want 200", noAuthRec.Code)
	}
}

// TestPublicVersion_ReturnsOnlyVersion — T044: GET /public/version returns
// only {"version": "..."}, no config, no health.
//
//nolint:paralleltest // serial: shared env
func TestPublicVersion_ReturnsOnlyVersion(t *testing.T) {
	ops, auth, _ := newAdminOpsHandler(t)

	rec := opsGet(t, ops, auth, nil, "/api/v1/public/version")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}

	var v publicVersionDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if v.Version == "" {
		t.Error("version is empty")
	}

	// The response must not contain config or clusters fields.
	body := rec.Body.String()
	if strings.Contains(body, "\"config\"") {
		t.Error("public version response contains 'config' — should not")
	}

	if strings.Contains(body, "\"clusters\"") {
		t.Error("public version response contains 'clusters' — should not")
	}
}
