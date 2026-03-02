package apiv1

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSearchVMs_OfflineMode(t *testing.T) {
	sm := newTestSM("testsecretthatis32byteslongexact!!")
	sm.offline = true
	h := NewSearchHandler(sm)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/search/vms", nil)
	signed := signToken(t, "testsecretthatis32byteslongexact!!", "testuser", false, accessTokenTTL)
	req.AddCookie(&http.Cookie{Name: accessTokenCookie, Value: signed})
	rr := httptest.NewRecorder()

	JWTMiddleware(sm, http.HandlerFunc(h.SearchVMs)).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp SearchResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Total != 0 || len(resp.Results) != 0 {
		t.Errorf("expected empty results in offline mode, got %d", resp.Total)
	}
}

func TestSearchVMs_Unauthorized(t *testing.T) {
	sm := newTestSM("testsecretthatis32byteslongexact!!")
	sm.offline = true
	h := NewSearchHandler(sm)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/search/vms", nil)
	rr := httptest.NewRecorder()

	JWTMiddleware(sm, http.HandlerFunc(h.SearchVMs)).ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}
