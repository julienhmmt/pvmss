package httpapi_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

const (
	badJSONBody       = "{bad"
	adminProfilesPath = "/api/v1/admin/profiles"
	adminTagsPath     = "/api/v1/admin/tags"
)

// Profiles, cloud-init templates and tags share the admin CRUD error shape:
// a malformed body is invalid_request, an unknown id is not_found, and domain
// validation failures carry their own machine code.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminCatalogCRUD_ErrorCodes(t *testing.T) {
	handler, authHandler := newMultiClusterAdminCatalogHandler(t)
	cookie := adminCookie(t, authHandler)

	cases := []struct {
		name   string
		method string
		path   string
		body   string
		status int
		code   string
	}{
		{"profile create bad body", http.MethodPost, adminProfilesPath, badJSONBody, http.StatusBadRequest, apiCodeInvalidRequest},
		{"profile create blank label", http.MethodPost, adminProfilesPath, `{"cluster":"default","label":""}`, http.StatusBadRequest, "invalid_profile"},
		{"profile update bad body", http.MethodPut, adminProfilesPath + "/nope", badJSONBody, http.StatusBadRequest, apiCodeInvalidRequest},
		{"profile delete unknown", http.MethodDelete, adminProfilesPath + "/nope?cluster=default", "", http.StatusNotFound, apiCodeNotFound},
		{"profile toggle unknown", http.MethodPost, adminProfilesPath + "/nope/toggle", `{"cluster":"default","enabled":true}`, http.StatusNotFound, apiCodeNotFound},
		{"template create bad body", http.MethodPost, "/api/v1/admin/cloudinit-templates", badJSONBody, http.StatusBadRequest, apiCodeInvalidRequest},
		{"template create bad content", http.MethodPost, "/api/v1/admin/cloudinit-templates", `{"cluster":"default","label":"x","content":"x"}`, http.StatusBadRequest, "invalid_content"},
		{"template update bad body", http.MethodPut, "/api/v1/admin/cloudinit-templates/nope", badJSONBody, http.StatusBadRequest, apiCodeInvalidRequest},
		{"template update unknown", http.MethodPut, "/api/v1/admin/cloudinit-templates/nope", `{"cluster":"default","label":"x","content":"#cloud-config"}`, http.StatusNotFound, apiCodeNotFound},
		{"template delete unknown", http.MethodDelete, "/api/v1/admin/cloudinit-templates/nope?cluster=default", "", http.StatusNotFound, apiCodeNotFound},
		{"template toggle unknown", http.MethodPost, "/api/v1/admin/cloudinit-templates/nope/toggle", `{"cluster":"default","enabled":true}`, http.StatusNotFound, apiCodeNotFound},
		{"tag create bad body", http.MethodPost, adminTagsPath, badJSONBody, http.StatusBadRequest, apiCodeInvalidRequest},
		{"tag create bad name", http.MethodPost, adminTagsPath, `{"cluster":"default","name":"BAD NAME!"}`, http.StatusBadRequest, "invalid_tag_name"},
		{"tag color unknown", http.MethodPut, adminTagsPath + "/nope/color", `{"cluster":"default","color":"#ffffff"}`, http.StatusNotFound, apiCodeNotFound},
		{"tag color bad body", http.MethodPut, adminTagsPath + "/nope/color", badJSONBody, http.StatusBadRequest, apiCodeInvalidRequest},
		{"tag delete unknown", http.MethodDelete, adminTagsPath + "/nope?cluster=default", "", http.StatusNotFound, apiCodeNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var rec interface {
				Bytes() []byte
			}

			var status int

			switch tc.method {
			case http.MethodPost:
				r := adminPost(t, handler, authHandler, cookie, tc.path, tc.body)
				rec, status = r.Body, r.Code
			case http.MethodPut:
				r := adminPut(t, handler, authHandler, cookie, tc.path, tc.body)
				rec, status = r.Body, r.Code
			default:
				r := adminDelete(t, handler, authHandler, cookie, tc.path)
				rec, status = r.Body, r.Code
			}

			if status != tc.status {
				t.Fatalf("status = %d, want %d: %s", status, tc.status, rec.Bytes())
			}

			var body struct {
				Code string `json:"code"`
			}
			if err := json.Unmarshal(rec.Bytes(), &body); err != nil {
				t.Fatalf("decode %q: %v", rec.Bytes(), err)
			}

			if body.Code != tc.code {
				t.Fatalf("code = %q, want %q", body.Code, tc.code)
			}
		})
	}
}
