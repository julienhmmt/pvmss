package httpapi_test

import (
	"context"
	"log/slog"
	"net/http"
	"path/filepath"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/config"
	"pvmss/server/internal/httpapi"
	"pvmss/server/internal/store"
	"pvmss/server/internal/testfixture"
	"testing"
)

// caplessClient satisfies only cluster.Client - no create capabilities.
type caplessClient struct{ cluster.Client }

// creatorOnlyClient implements cluster.Creator but not the write interfaces.
type creatorOnlyClient struct{ cluster.Client }

func (creatorOnlyClient) NextVMID(context.Context) (int, error) { return 0, nil }
func (creatorOnlyClient) CreateVM(context.Context, cluster.VMSpec) (string, error) {
	return "", nil
}
func (creatorOnlyClient) HostCPUFlags(context.Context) ([][]string, error) { return nil, nil }
func (creatorOnlyClient) CloneVM(context.Context, cluster.CloneSpec) (string, error) {
	return "", nil
}

func (creatorOnlyClient) TaskStatus(context.Context, string) (cluster.TaskStatus, error) {
	return cluster.TaskStatus{}, nil
}

// creatorWriterClient adds HardwareUpdater; FreeSpaceChecker comes free from
// the embedded cluster.Client, so the last capability gate is structural.
type creatorWriterClient struct{ creatorOnlyClient }

func (creatorWriterClient) UpdateHardware(context.Context, string, int, int, int, int, []string) error {
	return nil
}

func (creatorWriterClient) SetTags(context.Context, string, int, []string) error { return nil }

func (creatorWriterClient) ResizeDisk(context.Context, string, int, string, int) error { return nil }

func (creatorWriterClient) Action(context.Context, string, int, string) error { return nil }

func (creatorWriterClient) Delete(context.Context, string, int) error { return nil }

func newTargetStore(t *testing.T) *store.Store {
	t.Helper()

	st, err := store.Open(config.Configuration{
		DBPath:    filepath.Join(t.TempDir(), "vm-target.db"),
		LogLevel:  snapshotTestLogLevel,
		LogFormat: snapshotTestLogFormat,
		LogOutput: snapshotTestLogOutput,
	})
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}

	t.Cleanup(func() { _ = st.Close() })
	testfixture.SeedDemoFixtures(t, st)

	return st
}

// TestVMCreate_RequiresClusterWhenAmbiguous - an admin request that names no
// cluster on a multi-cluster provider is refused before any client call.
func TestVMCreate_RequiresClusterWhenAmbiguous(t *testing.T) {
	t.Parallel()

	handler, authHandler, _ := newVMCreateHandler(t)
	_ = handler

	multi := vmCreateClientProvider{clients: map[string]cluster.Client{
		dashCluster: &stubClusterClient{}, dashWestCluster: &stubClusterClient{},
	}}
	multiHandler := httpapi.NewVMCreateWithRegistry(authHandler, newTargetStore(t), multi, cluster.Fake{}, slog.New(slog.DiscardHandler), testPolicyService(newTargetStore(t), nil))

	response := postVMCreate(t, multiHandler, `{"name":"x"}`, adminCookie(t, authHandler))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 cluster_required", response.Code)
	}
}

// TestVMCreate_NilClientsUsesCreator - a handler without a client provider
// resolves the target from its own creator.
//
//nolint:paralleltest // serial: shared fake fixtures
func TestVMCreate_NilClientsUsesCreator(t *testing.T) {
	authHandler := newAuthHandler(t)
	handler := httpapi.NewVMCreateWithRegistry(authHandler, newTargetStore(t), nil, cluster.Fake{}, slog.New(slog.DiscardHandler), testPolicyService(newTargetStore(t), nil))

	response := postVMCreate(t, handler, `{"cluster":"default","name":"x"}`, aliceCookie(t, authHandler))
	if response.Code == http.StatusInternalServerError && response.Body.Len() == 0 {
		t.Fatal("nil clients must resolve through the handler creator")
	}

	if response.Code == http.StatusNotFound {
		t.Fatalf("status = %d, nil-client path must not 404", response.Code)
	}
}

// TestVMCreate_ClusterClientMissing - a named cluster with no client answers 404.
func TestVMCreate_ClusterClientMissing(t *testing.T) {
	t.Parallel()

	authHandler := newAuthHandler(t)
	empty := vmCreateClientProvider{clients: map[string]cluster.Client{}}
	handler := httpapi.NewVMCreateWithRegistry(authHandler, newTargetStore(t), empty, cluster.Fake{}, slog.New(slog.DiscardHandler), testPolicyService(newTargetStore(t), nil))

	response := postVMCreate(t, handler, `{"cluster":"default","name":"x"}`, aliceCookie(t, authHandler))
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", response.Code)
	}
}

// TestVMCreate_ClientCapabilityGates - a client missing a required create
// capability answers 500 before any cluster call; a fully capable client
// passes the gates and reaches request validation.
//
//nolint:paralleltest // serial: shared fake fixtures
func TestVMCreate_ClientCapabilityGates(t *testing.T) {
	for _, testCase := range []struct {
		name   string
		client cluster.Client
		want   int
	}{
		{"no creator", caplessClient{}, http.StatusInternalServerError},
		{"no hardware updater", creatorOnlyClient{}, http.StatusInternalServerError},
		{"all capabilities", creatorWriterClient{}, http.StatusBadRequest},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			handler, authHandler, _ := newVMCreateHandlerWithClient(t, testCase.client)

			response := postVMCreate(t, handler, `{"cluster":"default","name":"x"}`, aliceCookie(t, authHandler))
			if response.Code != testCase.want {
				t.Fatalf("status = %d, want %d", response.Code, testCase.want)
			}
		})
	}
}
