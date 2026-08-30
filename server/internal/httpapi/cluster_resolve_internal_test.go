package httpapi

import (
	"context"
	"fmt"
	"log/slog"
	"pvmss/server/internal/vm"
	"testing"
	"time"
)

const (
	testRefresherFallbackLabel = "fallback"
	testRefresherClusterALabel = "cluster-a"
	testRefresherClusterBLabel = "cluster-b"
)

// mockRefresher is a test double for vm.IndexRefresher that carries a label so
// the caller can assert which refresher was returned.
type mockRefresher struct {
	label string
}

func (m mockRefresher) Refresh(_ context.Context) (time.Time, error) {
	return time.Now(), nil
}

// mockRefresherResolver is a test double for ClusterRefresherResolver.
type mockRefresherResolver struct {
	refreshers map[string]vm.IndexRefresher
}

func (m mockRefresherResolver) RefresherFor(cluster string) (vm.IndexRefresher, error) {
	if r, ok := m.refreshers[cluster]; ok {
		return r, nil
	}

	return nil, fmt.Errorf("cluster %q not found", cluster)
}

// TestVMDetail_RefresherFor_ResolvedByCluster is a table-driven test on
// refresherFor: a known cluster returns that cluster's refresher; an unknown
// cluster returns the fallback (never nil).
func TestVMDetail_RefresherFor_ResolvedByCluster(t *testing.T) {
	t.Parallel()

	fallback := mockRefresher{label: testRefresherFallbackLabel}
	clusterA := mockRefresher{label: testRefresherClusterALabel}

	h := &VMDetail{
		refresher: fallback,
		refreshers: mockRefresherResolver{
			refreshers: map[string]vm.IndexRefresher{testRefresherClusterALabel: clusterA},
		},
		log: slog.Default(),
	}

	tests := []struct {
		name      string
		cluster   string
		wantLabel string
	}{
		{name: "known cluster returns its refresher", cluster: testRefresherClusterALabel, wantLabel: testRefresherClusterALabel},
		{name: "unknown cluster returns fallback", cluster: testRefresherClusterBLabel, wantLabel: testRefresherFallbackLabel},
		{name: "empty cluster name returns fallback", cluster: "", wantLabel: testRefresherFallbackLabel},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := h.refresherFor(tt.cluster)
			if got == nil {
				t.Fatal("refresherFor returned nil, want non-nil")
			}

			mock, ok := got.(mockRefresher)
			if !ok {
				t.Fatalf("refresherFor returned %T, want mockRefresher", got)
			}

			if mock.label != tt.wantLabel {
				t.Errorf("refresherFor(%q) label = %q, want %q", tt.cluster, mock.label, tt.wantLabel)
			}
		})
	}
}

// TestVMDetail_RefresherFor_SingleClusterFallback verifies that when no
// ClusterRefresherResolver is set (single-cluster mode), the fallback
// refresher is always returned regardless of the cluster name.
func TestVMDetail_RefresherFor_SingleClusterFallback(t *testing.T) {
	t.Parallel()

	fallback := mockRefresher{label: testRefresherFallbackLabel}
	h := &VMDetail{
		refresher: fallback,
		log:       slog.Default(),
	}

	got := h.refresherFor("any-cluster")
	if got == nil {
		t.Fatal("refresherFor returned nil, want non-nil")
	}

	mock, ok := got.(mockRefresher)
	if !ok {
		t.Fatalf("refresherFor returned %T, want mockRefresher", got)
	}

	if mock.label != testRefresherFallbackLabel {
		t.Errorf("refresherFor label = %q, want %s", mock.label, testRefresherFallbackLabel)
	}
}

// TestVMCloudInit_RefresherFor_ResolvedByCluster is the same table-driven
// test for VMCloudInit's refresherFor.
func TestVMCloudInit_RefresherFor_ResolvedByCluster(t *testing.T) {
	t.Parallel()

	fallback := mockRefresher{label: testRefresherFallbackLabel}
	clusterB := mockRefresher{label: testRefresherClusterBLabel}

	h := &VMCloudInit{
		refresher: fallback,
		refreshers: mockRefresherResolver{
			refreshers: map[string]vm.IndexRefresher{testRefresherClusterBLabel: clusterB},
		},
		log: slog.Default(),
	}

	tests := []struct {
		name      string
		cluster   string
		wantLabel string
	}{
		{name: "known cluster returns its refresher", cluster: testRefresherClusterBLabel, wantLabel: testRefresherClusterBLabel},
		{name: "unknown cluster returns fallback", cluster: "cluster-z", wantLabel: testRefresherFallbackLabel},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := h.refresherFor(tt.cluster)
			if got == nil {
				t.Fatal("refresherFor returned nil, want non-nil")
			}

			mock, ok := got.(mockRefresher)
			if !ok {
				t.Fatalf("refresherFor returned %T, want mockRefresher", got)
			}

			if mock.label != tt.wantLabel {
				t.Errorf("refresherFor(%q) label = %q, want %q", tt.cluster, mock.label, tt.wantLabel)
			}
		})
	}
}
