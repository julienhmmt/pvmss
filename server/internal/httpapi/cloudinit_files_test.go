package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"pvmss/server/internal/httpapi"
	"strings"
	"testing"
)

// newCloudInitFilesFixture builds the handler over a real store + real auth,
// and returns signed-in cookies for alice and bob.
func newCloudInitFilesFixture(t *testing.T) (*httpapi.CloudInitFiles, *http.Cookie, *http.Cookie) {
	t.Helper()

	authHandler, st := newAuthHandlerWithStore(t)
	handler := httpapi.NewCloudInitFiles(authHandler, st, testLogger(t))

	alice := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice","cluster":"default"}`)
	bob := loginCookie(t, authHandler, `{"username":"bob","password":"pvmss-bob","cluster":"default"}`)

	return handler, alice, bob
}

func cloudInitFileRequest(t *testing.T, method, path, body string, cookie *http.Cookie) *http.Request {
	t.Helper()

	req := httptest.NewRequestWithContext(context.Background(), method, path, strings.NewReader(body))
	req.AddCookie(cookie)

	return req
}

// createAliceFile posts one file as alice and returns the created DTO.
func createAliceFile(t *testing.T, h *httpapi.CloudInitFiles, alice *http.Cookie, label, content string) map[string]any {
	t.Helper()

	rec := httptest.NewRecorder()
	h.ServeCreate(rec, cloudInitFileRequest(t, http.MethodPost, "/api/v1/cloudinit/files", `{"label":"`+label+`","content":"`+content+`"}`, alice))

	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201: %s", rec.Code, rec.Body.String())
	}

	var dto map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &dto); err != nil {
		t.Fatalf("decode created file: %v", err)
	}

	return dto
}

func TestCloudInitFiles_CRUD(t *testing.T) {
	t.Parallel()

	handler, alice, _ := newCloudInitFilesFixture(t)
	content := "#cloud-config\\npackages:\\n  - htop\\n"

	created := createAliceFile(t, handler, alice, "Dev box", content)

	id, _ := created["id"].(string)
	if id != "dev-box" {
		t.Fatalf("created id = %v, want dev-box", created["id"])
	}

	// List shows the file without its content.
	rec := httptest.NewRecorder()
	handler.ServeList(rec, cloudInitFileRequest(t, http.MethodGet, "/api/v1/cloudinit/files", "", alice))

	var list struct {
		Files []struct {
			ID      string `json:"id"`
			Label   string `json:"label"`
			Content string `json:"content"`
		} `json:"files"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list: %v", err)
	}

	if len(list.Files) != 1 || list.Files[0].ID != "dev-box" {
		t.Fatalf("list = %+v, want one dev-box row", list.Files)
	}

	if list.Files[0].Content != "" {
		t.Error("list leaked file content - list rows carry id/label/updatedAt only")
	}

	// Get returns the full row.
	rec = httptest.NewRecorder()
	getReq := cloudInitFileRequest(t, http.MethodGet, "/api/v1/cloudinit/files/dev-box", "", alice)
	getReq.SetPathValue("id", "dev-box")
	handler.ServeGet(rec, getReq)

	if rec.Code != http.StatusOK {
		t.Fatalf("get status = %d: %s", rec.Code, rec.Body.String())
	}

	var got struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode get: %v", err)
	}

	if !strings.Contains(got.Content, "htop") {
		t.Errorf("content = %q, want the stored document", got.Content)
	}

	// Update rewrites label + content.
	rec = httptest.NewRecorder()
	putReq := cloudInitFileRequest(t, http.MethodPut, "/api/v1/cloudinit/files/dev-box", `{"label":"Dev box v2","content":"`+content+`"}`, alice)
	putReq.SetPathValue("id", "dev-box")
	handler.ServeUpdate(rec, putReq)

	if rec.Code != http.StatusOK {
		t.Fatalf("update status = %d: %s", rec.Code, rec.Body.String())
	}

	// Delete removes it; a follow-up get is a 404.
	rec = httptest.NewRecorder()
	delReq := cloudInitFileRequest(t, http.MethodDelete, "/api/v1/cloudinit/files/dev-box", "", alice)
	delReq.SetPathValue("id", "dev-box")
	handler.ServeDelete(rec, delReq)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d: %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	goneReq := cloudInitFileRequest(t, http.MethodGet, "/api/v1/cloudinit/files/dev-box", "", alice)
	goneReq.SetPathValue("id", "dev-box")
	handler.ServeGet(rec, goneReq)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("get after delete status = %d, want 404", rec.Code)
	}
}

func TestCloudInitFiles_OwnerIsolation(t *testing.T) {
	t.Parallel()

	handler, alice, bob := newCloudInitFilesFixture(t)
	createAliceFile(t, handler, alice, "Dev box", "#cloud-config\\npackages:\\n  - htop\\n")

	// bob's list is empty; every bob verb on alice's id is a 404.
	rec := httptest.NewRecorder()
	handler.ServeList(rec, cloudInitFileRequest(t, http.MethodGet, "/api/v1/cloudinit/files", "", bob))

	var list struct {
		Files []struct {
			ID string `json:"id"`
		} `json:"files"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode bob list: %v", err)
	}

	if len(list.Files) != 0 {
		t.Fatalf("bob sees alice's files: %+v", list.Files)
	}

	for _, tc := range []struct {
		name   string
		method string
		body   string
		serve  func(http.ResponseWriter, *http.Request)
	}{
		{"get", http.MethodGet, "", handler.ServeGet},
		{"update", http.MethodPut, `{"label":"x","content":"#cloud-config\\n"}`, handler.ServeUpdate},
		{"delete", http.MethodDelete, "", handler.ServeDelete},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			req := cloudInitFileRequest(t, tc.method, "/api/v1/cloudinit/files/dev-box", tc.body, bob)
			req.SetPathValue("id", "dev-box")
			tc.serve(rec, req)

			if rec.Code != http.StatusNotFound {
				t.Fatalf("bob %s alice's file: status = %d, want 404", tc.name, rec.Code)
			}

			assertAPIError(t, rec.Body.Bytes(), "not_found")
		})
	}
}

func TestCloudInitFiles_Unauthenticated(t *testing.T) {
	t.Parallel()

	handler, _, _ := newCloudInitFilesFixture(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/cloudinit/files", nil)
	handler.ServeList(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestCloudInitFiles_Errors(t *testing.T) {
	t.Parallel()

	handler, alice, _ := newCloudInitFilesFixture(t)

	// Invalid content → 400 invalid_cloudinit_file carrying the detail.
	rec := httptest.NewRecorder()
	handler.ServeCreate(rec, cloudInitFileRequest(t, http.MethodPost, "/api/v1/cloudinit/files", `{"label":"Bad","content":"not cloud-config"}`, alice))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid content status = %d, want 400", rec.Code)
	}

	assertAPIError(t, rec.Body.Bytes(), "invalid_cloudinit_file")

	// Duplicate label → same slug → 409 duplicate_cloudinit_file.
	createAliceFile(t, handler, alice, "Dev box", "#cloud-config\\n")

	rec = httptest.NewRecorder()
	handler.ServeCreate(rec, cloudInitFileRequest(t, http.MethodPost, "/api/v1/cloudinit/files", `{"label":"dev  box","content":"#cloud-config\\n"}`, alice))

	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate status = %d, want 409", rec.Code)
	}

	assertAPIError(t, rec.Body.Bytes(), "duplicate_cloudinit_file")

	// Body over the limit → 400.
	big := `{"label":"Big","content":"#cloud-config\n` + strings.Repeat("x", 20*1024) + `"}`

	rec = httptest.NewRecorder()
	handler.ServeCreate(rec, cloudInitFileRequest(t, http.MethodPost, "/api/v1/cloudinit/files", big, alice))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("oversized body status = %d, want 400", rec.Code)
	}
}
