package apiv1

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/julienschmidt/httprouter"
)

func TestAdminListTags(t *testing.T) {
	secret := "testsecretthatis32byteslongexact!!"
	sm := newAdminTestSM(secret, true)
	sm.settings.Tags = []string{"pvmss", "dev", "prod"}
	handler := MakeAdminMutationsHandler(sm)
	h := JWTAdminMiddleware(sm, http.HandlerFunc(handler.ListTags))

	token := signToken(t, secret, "admin", true, 15*time.Minute)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/tags", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: token})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var tags []AdminTagResponse
	if err := json.NewDecoder(rr.Body).Decode(&tags); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if len(tags) != 3 {
		t.Errorf("expected 3 tags, got %d", len(tags))
	}
}

func TestAdminCreateTag(t *testing.T) {
	secret := "testsecretthatis32byteslongexact!!"
	sm := newAdminTestSM(secret, true)
	sm.settings.Tags = []string{"pvmss"}
	handler := MakeAdminMutationsHandler(sm)
	h := JWTAdminMiddleware(sm, http.HandlerFunc(handler.CreateTag))

	body, _ := json.Marshal(CreateTagRequest{Name: "newtag"})
	token := signToken(t, secret, "admin", true, 15*time.Minute)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/tags", bytes.NewReader(body))
	req.AddCookie(&http.Cookie{Name: "access_token", Value: token})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	// Verify settings updated
	if len(sm.settings.Tags) != 2 {
		t.Errorf("expected 2 tags after create, got %d", len(sm.settings.Tags))
	}
}

func TestAdminCreateTag_Duplicate(t *testing.T) {
	secret := "testsecretthatis32byteslongexact!!"
	sm := newAdminTestSM(secret, true)
	sm.settings.Tags = []string{"pvmss"}
	handler := MakeAdminMutationsHandler(sm)
	h := JWTAdminMiddleware(sm, http.HandlerFunc(handler.CreateTag))

	body, _ := json.Marshal(CreateTagRequest{Name: "pvmss"})
	token := signToken(t, secret, "admin", true, 15*time.Minute)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/tags", bytes.NewReader(body))
	req.AddCookie(&http.Cookie{Name: "access_token", Value: token})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for duplicate tag, got %d", rr.Code)
	}
}

func TestAdminDeleteTag(t *testing.T) {
	secret := "testsecretthatis32byteslongexact!!"
	sm := newAdminTestSM(secret, true)
	sm.settings.Tags = []string{"pvmss", "removeme"}
	handler := MakeAdminMutationsHandler(sm)
	h := JWTAdminMiddleware(sm, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Inject httprouter params into context
		ps := httprouter.Params{{Key: "name", Value: "removeme"}}
		ctx := context.WithValue(r.Context(), httprouter.ParamsKey, ps)
		handler.DeleteTag(w, r.WithContext(ctx))
	}))

	token := signToken(t, secret, "admin", true, 15*time.Minute)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/tags/removeme", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: token})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rr.Code, rr.Body.String())
	}
	if len(sm.settings.Tags) != 1 {
		t.Errorf("expected 1 tag after delete, got %d", len(sm.settings.Tags))
	}
}

func TestAdminGetLimits(t *testing.T) {
	secret := "testsecretthatis32byteslongexact!!"
	sm := newAdminTestSM(secret, true)
	handler := MakeAdminMutationsHandler(sm)
	h := JWTAdminMiddleware(sm, http.HandlerFunc(handler.GetLimits))

	token := signToken(t, secret, "admin", true, 15*time.Minute)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/limits", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: token})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var limits AdminLimitsResponse
	if err := json.NewDecoder(rr.Body).Decode(&limits); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if limits.VM.Cores.Max != 4 {
		t.Errorf("expected VM cores max=4, got %d", limits.VM.Cores.Max)
	}
	if limits.MaxVMPerUser != 5 {
		t.Errorf("expected max_vm_per_user=5, got %d", limits.MaxVMPerUser)
	}
}

func TestAdminListCloudInit(t *testing.T) {
	secret := "testsecretthatis32byteslongexact!!"
	sm := newAdminTestSM(secret, true)
	handler := MakeAdminMutationsHandler(sm)
	h := JWTAdminMiddleware(sm, http.HandlerFunc(handler.ListCloudInit))

	token := signToken(t, secret, "admin", true, 15*time.Minute)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/cloudinit", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: token})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var items []AdminCloudInitResponse
	if err := json.NewDecoder(rr.Body).Decode(&items); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 cloud-init templates, got %d", len(items))
	}
}

func TestAdminCreateCloudInit(t *testing.T) {
	secret := "testsecretthatis32byteslongexact!!"
	sm := newAdminTestSM(secret, true)
	handler := MakeAdminMutationsHandler(sm)
	h := JWTAdminMiddleware(sm, http.HandlerFunc(handler.CreateCloudInit))

	body, _ := json.Marshal(CreateCloudInitRequest{
		Name:        "Test Template",
		Description: "A test",
		Storage:     "local",
		YAMLContent: "#cloud-config\npackages:\n  - vim",
	})
	token := signToken(t, secret, "admin", true, 15*time.Minute)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/cloudinit", bytes.NewReader(body))
	req.AddCookie(&http.Cookie{Name: "access_token", Value: token})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	if len(sm.settings.CloudInitTemplates) != 1 {
		t.Errorf("expected 1 template, got %d", len(sm.settings.CloudInitTemplates))
	}
	if sm.settings.CloudInitTemplates[0].ID != "test-template" {
		t.Errorf("expected ID 'test-template', got %q", sm.settings.CloudInitTemplates[0].ID)
	}
}
