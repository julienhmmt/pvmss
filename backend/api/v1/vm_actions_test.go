package apiv1

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/julienschmidt/httprouter"
)

// withVMID injects a httprouter :id param into the request context.
func withVMID(req *http.Request, id string) *http.Request {
	ctx := context.WithValue(req.Context(), httprouter.ParamsKey, httprouter.Params{{Key: "id", Value: id}})
	return req.WithContext(ctx)
}

func TestVMAction_InvalidAction(t *testing.T) {
	sm := newTestSM("testsecretthatis32byteslongexact!!")
	sm.offline = true
	h := NewVMActionHandler(sm)

	body, _ := json.Marshal(VMActionRequest{Action: "fly", Node: "node1"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/vms/100/action", io.NopCloser(bytes.NewReader(body)))
	req = withVMID(req, "100")
	req.Header.Set("Content-Type", "application/json")
	signed := signToken(t, "testsecretthatis32byteslongexact!!", "testuser", false, accessTokenTTL)
	req.AddCookie(&http.Cookie{Name: accessTokenCookie, Value: signed})

	rr := httptest.NewRecorder()
	JWTMiddleware(sm, http.HandlerFunc(h.VMAction)).ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid action, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestVMAction_OfflineMode(t *testing.T) {
	sm := newTestSM("testsecretthatis32byteslongexact!!")
	sm.offline = true
	h := NewVMActionHandler(sm)

	body, _ := json.Marshal(VMActionRequest{Action: "start", Node: "node1"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/vms/100/action", io.NopCloser(bytes.NewReader(body)))
	req = withVMID(req, "100")
	req.Header.Set("Content-Type", "application/json")
	signed := signToken(t, "testsecretthatis32byteslongexact!!", "testuser", false, accessTokenTTL)
	req.AddCookie(&http.Cookie{Name: accessTokenCookie, Value: signed})

	rr := httptest.NewRecorder()
	JWTMiddleware(sm, http.HandlerFunc(h.VMAction)).ServeHTTP(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 in offline mode, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestVMAction_MissingNode(t *testing.T) {
	sm := newTestSM("testsecretthatis32byteslongexact!!")
	sm.offline = true
	h := NewVMActionHandler(sm)

	body, _ := json.Marshal(VMActionRequest{Action: "start", Node: ""})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/vms/100/action", io.NopCloser(bytes.NewReader(body)))
	req = withVMID(req, "100")
	req.Header.Set("Content-Type", "application/json")
	signed := signToken(t, "testsecretthatis32byteslongexact!!", "testuser", false, accessTokenTTL)
	req.AddCookie(&http.Cookie{Name: accessTokenCookie, Value: signed})

	rr := httptest.NewRecorder()
	JWTMiddleware(sm, http.HandlerFunc(h.VMAction)).ServeHTTP(rr, req)
	// node check comes before offline check, so 400
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing node, got %d: %s", rr.Code, rr.Body.String())
	}
}
