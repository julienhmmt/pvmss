package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/httpapi"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/policy"
	"strings"
	"testing"
)

func TestAdminPolicy_RequiresAdminAndReturnsSeparatedShape(t *testing.T) {
	policyHandler, authHandler := newPolicyHandler(t)
	mux := policyMux(policyHandler, authHandler)
	userCookie := aliceCookie(t, authHandler)
	adminCookie := adminCookie(t, authHandler)

	for _, testCase := range []struct {
		name   string
		cookie *http.Cookie
		status int
	}{
		{name: "user", cookie: userCookie, status: http.StatusForbidden},
		{name: "admin", cookie: adminCookie, status: http.StatusOK},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/admin/policy?cluster=default", nil)
			request.AddCookie(testCase.cookie)
			recorder := httptest.NewRecorder()
			mux.ServeHTTP(recorder, request)
			if recorder.Code != testCase.status {
				t.Fatalf("status = %d, want %d: %s", recorder.Code, testCase.status, recorder.Body.String())
			}
			if testCase.status != http.StatusOK {
				return
			}
			var response struct {
				Cluster string `json:"cluster"`
				Gabarit struct {
					MaxDiskPerVMGB  int  `json:"maxDiskPerVmGb"`
					AllowCustomYaml bool `json:"allowCustomYaml"`
				} `json:"gabarit"`
				Quota struct {
					MaxVMPerUser int `json:"maxVmPerUser"`
				} `json:"quota"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Fatalf("decode policy: %v", err)
			}
			if response.Cluster != "default" || response.Gabarit.MaxDiskPerVMGB != 500 || !response.Gabarit.AllowCustomYaml || response.Quota.MaxVMPerUser != -1 {
				t.Fatalf("response = %+v", response)
			}
		})
	}
}

func TestAdminPolicy_PutPartialUpdateAndRejectsInvalid(t *testing.T) {
	policyHandler, authHandler := newPolicyHandler(t)
	mux := policyMux(policyHandler, authHandler)
	cookie := adminCookie(t, authHandler)
	request := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/v1/admin/policy", strings.NewReader(`{"cluster":"default","gabarit":{"maxDiskPerVmGb":10},"quota":{"maxVmPerUser":1}}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(cookie)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("put status = %d: %s", recorder.Code, recorder.Body.String())
	}
	var response map[string]json.RawMessage
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode put: %v", err)
	}
	if !bytes.Contains(response["gabarit"], []byte(`"maxDiskPerVmGb":10`)) || !bytes.Contains(response["quota"], []byte(`"maxVmPerUser":1`)) {
		t.Fatalf("updated response = %s", recorder.Body.String())
	}
	invalid := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/v1/admin/policy", strings.NewReader(`{"cluster":"default","gabarit":{"maxCores":-1}}`))
	invalid.Header.Set("Content-Type", "application/json")
	invalid.AddCookie(cookie)
	invalidRecorder := httptest.NewRecorder()
	mux.ServeHTTP(invalidRecorder, invalid)
	if invalidRecorder.Code != http.StatusBadRequest {
		t.Fatalf("invalid status = %d, want 400", invalidRecorder.Code)
	}
}

func TestAdminPolicyNodes_ListsUsageAndValidatesWrites(t *testing.T) {
	policyHandler, authHandler := newPolicyHandler(t)
	mux := policyMux(policyHandler, authHandler)
	userCookie := aliceCookie(t, authHandler)
	userRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/admin/policy/nodes?cluster=default", nil)
	userRequest.AddCookie(userCookie)
	userRecorder := httptest.NewRecorder()
	mux.ServeHTTP(userRecorder, userRequest)
	if userRecorder.Code != http.StatusForbidden {
		t.Fatalf("non-admin node list status = %d, want 403", userRecorder.Code)
	}
	userWrite := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/v1/admin/policy/nodes/pve-node-02", strings.NewReader(`{"cluster":"default","maxVcpus":0}`))
	userWrite.Header.Set("Content-Type", "application/json")
	userWrite.AddCookie(userCookie)
	userWriteRecorder := httptest.NewRecorder()
	mux.ServeHTTP(userWriteRecorder, userWrite)
	if userWriteRecorder.Code != http.StatusForbidden {
		t.Fatalf("non-admin node write status = %d, want 403", userWriteRecorder.Code)
	}
	cookie := adminCookie(t, authHandler)
	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/admin/policy/nodes?cluster=default", nil)
	request.AddCookie(cookie)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !bytes.Contains(recorder.Body.Bytes(), []byte(`"node":"pve-node-01"`)) {
		t.Fatalf("nodes response = %d %s", recorder.Code, recorder.Body.String())
	}
	var nodes []struct {
		Node          string `json:"node"`
		UsedVCPUs     int    `json:"usedVcpus"`
		PhysicalVCPUs int    `json:"physicalVcpus"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &nodes); err != nil || len(nodes) == 0 {
		t.Fatalf("decode nodes = %v, body = %s", err, recorder.Body.String())
	}
	var targetNode string
	var usedVCPUs int
	for _, node := range nodes {
		if node.Node == "pve-node-02" {
			targetNode, usedVCPUs = node.Node, node.UsedVCPUs
		}
	}
	if targetNode == "" {
		t.Fatal("pve-node-02 missing from node list")
	}
	below := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/v1/admin/policy/nodes/"+targetNode, strings.NewReader(`{"cluster":"default","maxVcpus":1}`))
	below.Header.Set("Content-Type", "application/json")
	below.AddCookie(cookie)
	belowRecorder := httptest.NewRecorder()
	mux.ServeHTTP(belowRecorder, below)
	if usedVCPUs > 1 && belowRecorder.Code != http.StatusBadRequest {
		t.Fatalf("below-usage status = %d, want 400", belowRecorder.Code)
	}
	above := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/v1/admin/policy/nodes/"+targetNode, strings.NewReader(`{"cluster":"default","maxVcpus":9999}`))
	above.Header.Set("Content-Type", "application/json")
	above.AddCookie(cookie)
	aboveRecorder := httptest.NewRecorder()
	mux.ServeHTTP(aboveRecorder, above)
	if aboveRecorder.Code != http.StatusBadRequest {
		t.Fatalf("above-physical status = %d, want 400", aboveRecorder.Code)
	}
	zero := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/v1/admin/policy/nodes/"+targetNode, strings.NewReader(`{"cluster":"default","maxVcpus":0}`))
	zero.Header.Set("Content-Type", "application/json")
	zero.AddCookie(cookie)
	zeroRecorder := httptest.NewRecorder()
	mux.ServeHTTP(zeroRecorder, zero)
	if zeroRecorder.Code != http.StatusOK {
		t.Fatalf("zero capacity status = %d, want 200: %s", zeroRecorder.Code, zeroRecorder.Body.String())
	}
}

func newPolicyHandler(t *testing.T) (*httpapi.AdminPolicy, *httpapi.Auth) {
	t.Helper()
	st := newAdminStore(t)
	authHandler := newAuthHandler(t)
	fake := cluster.Fake{}
	snapshot, err := fake.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	index := inventory.BuildIndex(snapshot)
	service := policy.New(st, inventory.NewProjectionFromIndex(&index), fake)
	return httpapi.NewAdminPolicy(authHandler, service, slog.New(slog.DiscardHandler)), authHandler
}

func policyMux(handler *httpapi.AdminPolicy, authHandler *httpapi.Auth) *http.ServeMux {
	mux := http.NewServeMux()
	guard := authHandler.RequireAdmin
	mux.Handle("GET /api/v1/admin/policy", guard(http.HandlerFunc(handler.ServePolicy)))
	mux.Handle("PUT /api/v1/admin/policy", guard(http.HandlerFunc(handler.ServePolicyUpdate)))
	mux.Handle("GET /api/v1/admin/policy/nodes", guard(http.HandlerFunc(handler.ServePolicyNodes)))
	mux.Handle("PUT /api/v1/admin/policy/nodes/{node}", guard(http.HandlerFunc(handler.ServePolicyNodeUpdate)))
	return mux
}
