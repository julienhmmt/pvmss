package httpapi_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"pvmss/server/internal/cluster"
	"pvmss/server/internal/httpapi"
)

type stubClusterClient struct {
	nodes []cluster.Node
	err   error
}

func (s stubClusterClient) ListNodes(_ context.Context) ([]cluster.Node, error) {
	return s.nodes, s.err
}

func (stubClusterClient) Authenticate(_ context.Context, _, _ string) (cluster.Identity, error) {
	return cluster.Identity{}, cluster.ErrNotImplemented
}

func (stubClusterClient) ChangePassword(_ context.Context, _, _, _ string) error {
	return cluster.ErrNotImplemented
}

func TestClusterNodes_Success(t *testing.T) {
	client := stubClusterClient{nodes: []cluster.Node{
		{
			Name:         "pve-node-01",
			Status:       cluster.NodeOnline,
			CPUCores:     32,
			CPUUsage:     0.42,
			MemoryTotal:  137438953472,
			MemoryUsed:   68719476736,
			StorageTotal: 2199023255552,
			StorageUsed:  879609302220,
		},
	}}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	h := httpapi.NewClusterNodes(client, logger)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/cluster/nodes", nil)
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var got struct {
		Nodes []struct {
			Name         string  `json:"name"`
			Status       string  `json:"status"`
			CPUCores     int     `json:"cpuCores"`
			CPUUsage     float64 `json:"cpuUsage"`
			MemoryTotal  int64   `json:"memoryTotal"`
			MemoryUsed   int64   `json:"memoryUsed"`
			StorageTotal int64   `json:"storageTotal"`
			StorageUsed  int64   `json:"storageUsed"`
		} `json:"nodes"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got.Nodes) != 1 {
		t.Fatalf("nodes count = %d, want 1", len(got.Nodes))
	}
	n := got.Nodes[0]
	if n.Name != "pve-node-01" || n.Status != "online" || n.CPUCores != 32 ||
		n.MemoryTotal != 137438953472 || n.MemoryUsed != 68719476736 ||
		n.StorageTotal != 2199023255552 || n.StorageUsed != 879609302220 {
		t.Fatalf("unexpected node shape: %+v", n)
	}
}

func TestClusterNodes_Unreachable(t *testing.T) {
	client := stubClusterClient{err: cluster.ErrUnreachable}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	h := httpapi.NewClusterNodes(client, logger)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/cluster/nodes", nil)
	h.ServeHTTP(w, r)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadGateway)
	}

	var got struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Code != "cluster_unreachable" {
		t.Fatalf("code = %q, want cluster_unreachable", got.Code)
	}
	forbidden := []string{"http://", "https://", "token", "password", "credential"}
	for _, f := range forbidden {
		if strings.Contains(got.Message, f) {
			t.Fatalf("message leaks driver detail: %q contains %q", got.Message, f)
		}
	}
}

func TestClusterNodes_EmptyIsOK(t *testing.T) {
	client := stubClusterClient{nodes: []cluster.Node{}}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	h := httpapi.NewClusterNodes(client, logger)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/cluster/nodes", nil)
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var got struct {
		Nodes []json.RawMessage `json:"nodes"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Nodes == nil || len(got.Nodes) != 0 {
		t.Fatalf("nodes = %v, want empty array not null", got.Nodes)
	}
}
