package httpapi_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"pvmss/server/internal/config"
	"pvmss/server/internal/httpapi"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/store"
	"testing"
	"time"
)

type onboardingStepDTO struct {
	Key        string `json:"key"`
	Done       bool   `json:"done"`
	Required   bool   `json:"required"`
	Count      int    `json:"count"`
	ClusterKey string `json:"clusterKey"`
	Manual     bool   `json:"manual"`
}

type onboardingDashboardDTO struct {
	Onboarding *struct {
		Steps []onboardingStepDTO `json:"steps"`
	} `json:"onboarding"`
}

const onboardingSessionSecret = "onboarding-session-secret-at-least-thirty-two-bytes"

type onboardingSeed struct {
	key  string
	seed func(ctx context.Context, st *store.Store) error
}

// onboardingSeeds brings the cluster to "ready for users", in step order.
var onboardingSeeds = []onboardingSeed{
	{"nodes", func(ctx context.Context, st *store.Store) error {
		return st.SetNodeEnabled(ctx, dashCluster, "pve1", true)
	}},
	{"storages", func(ctx context.Context, st *store.Store) error {
		return st.SetStorageEnabled(ctx, dashCluster, "local", "pve1", true)
	}},
	{"bridges", func(ctx context.Context, st *store.Store) error {
		return st.SetBridgeEnabled(ctx, dashCluster, "pve1", "vmbr0", true)
	}},
	{"images", func(ctx context.Context, st *store.Store) error {
		return st.SetISOEnabled(ctx, dashCluster, "pve1", "local", "debian.iso", true)
	}},
	{"profiles", func(ctx context.Context, st *store.Store) error {
		return st.InsertProfile(ctx, dashCluster, "small", store.ProfileValues{Label: "Small", Sockets: 1, CPUCores: 1, MemoryMB: 1024, DiskGB: 10, Bus: "scsi"})
	}},
	{"users", func(ctx context.Context, st *store.Store) error {
		return st.SetClusterOIDCRealm(ctx, dashCluster, "sso")
	}},
}

func onboardingDashboard(t *testing.T, st *store.Store) onboardingDashboardDTO {
	t.Helper()

	authHandler := newAuthHandler(t)
	ops := httpapi.NewAdminOps(authHandler, st, inventory.NewProjection(), "test", slog.New(slog.DiscardHandler))
	ops.SetInventorySource(inventory.NewRegistryFromIndexes(map[string]*inventory.Index{dashCluster: attentionIndex()}), time.Minute)

	rec := opsGet(t, ops, authHandler, adminCookie(t, authHandler), "/api/v1/admin/dashboard")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}

	var dash onboardingDashboardDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &dash); err != nil {
		t.Fatalf("decode: %v", err)
	}

	return dash
}

// assertOnboardingSteps checks that exactly the first seeded steps are done,
// that undone steps link to the cluster and that policy stays optional.
func assertOnboardingSteps(t *testing.T, steps []onboardingStepDTO, seeded int) {
	t.Helper()

	done := map[string]bool{}
	for _, seed := range onboardingSeeds[:seeded] {
		done[seed.key] = true
	}

	for _, step := range steps {
		if step.Key == "policy" {
			if step.Required || step.Done {
				t.Fatalf("policy step = %+v, want optional and not done", step)
			}

			continue
		}

		if step.Done != done[step.Key] {
			t.Fatalf("after %d seeds: step %q done = %v, want %v", seeded, step.Key, step.Done, done[step.Key])
		}

		if !step.Done && step.ClusterKey != dashCluster {
			t.Fatalf("step %q clusterKey = %q, want %q", step.Key, step.ClusterKey, dashCluster)
		}
	}
}

// openOnboardingStore opens a store with the dash cluster registered.
func openOnboardingStore(t *testing.T) *store.Store {
	t.Helper()

	st, err := store.Open(config.Configuration{
		DBPath:        filepath.Join(t.TempDir(), "dashboard-onboarding.db"),
		SessionSecret: onboardingSessionSecret,
	})
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}

	t.Cleanup(func() { _ = st.Close() })

	//nolint:gosec // deterministic test secret
	err = st.CreateCluster(context.Background(), store.ClusterRow{Name: dashCluster, URL: "https://pve.example.com:8006", TokenID: "pvmss@pve!svc", TokenSecret: "s"})
	if err != nil {
		t.Fatalf("create cluster: %v", err)
	}

	return st
}

//nolint:paralleltest // serial: shared fake dataset
func TestAdminDashboard_Onboarding(t *testing.T) {
	ctx := context.Background()
	st := openOnboardingStore(t)

	for i, seed := range onboardingSeeds {
		dash := onboardingDashboard(t, st)
		if dash.Onboarding == nil {
			t.Fatalf("before %q: onboarding missing", seed.key)
		}

		assertOnboardingSteps(t, dash.Onboarding.Steps, i)

		if err := seed.seed(ctx, st); err != nil {
			t.Fatalf("seed %q: %v", seed.key, err)
		}
	}

	// Every required step done: the block disappears though policy is unsaved.
	if dash := onboardingDashboard(t, st); dash.Onboarding != nil {
		t.Fatalf("onboarding = %+v, want nil once required steps are done", dash.Onboarding)
	}
}

// onboardingPut issues PUT /admin/onboarding and decodes the reply's
// onboarding block.
func onboardingPut(t *testing.T, st *store.Store, body string) (*struct {
	Steps []onboardingStepDTO `json:"steps"`
}, *httptest.ResponseRecorder,
) {
	t.Helper()

	authHandler := newAuthHandler(t)
	ops := httpapi.NewAdminOps(authHandler, st, inventory.NewProjection(), "test", slog.New(slog.DiscardHandler))
	ops.SetInventorySource(inventory.NewRegistryFromIndexes(map[string]*inventory.Index{dashCluster: attentionIndex()}), time.Minute)

	rec := opsPut(t, ops, authHandler, adminCookie(t, authHandler), "/api/v1/admin/onboarding", body)

	var resp struct {
		Onboarding *struct {
			Steps []onboardingStepDTO `json:"steps"`
		} `json:"onboarding"`
	}

	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
	}

	return resp.Onboarding, rec
}

//nolint:paralleltest // serial: shared fake dataset
func TestAdminOnboarding_MarkDoneRejectsInvalidKeys(t *testing.T) {
	ctx := context.Background()
	st := openOnboardingStore(t)

	// Unknown and empty keys are rejected, nothing is persisted.
	for _, body := range []string{`{"steps":["bogus"]}`, `{"steps":[]}`} {
		if _, rec := onboardingPut(t, st, body); rec.Code != http.StatusBadRequest {
			t.Fatalf("body %s: status = %d, want 400", body, rec.Code)
		}
	}

	if done, err := st.OnboardingDoneSteps(ctx); err != nil || len(done) != 0 {
		t.Fatalf("rejected writes still persisted %v, err %v", done, err)
	}
}

//nolint:paralleltest // serial: shared fake dataset
func TestAdminOnboarding_MarkDone(t *testing.T) {
	st := openOnboardingStore(t)

	// One bypassed step flips done with manual=true; the rest stays undone.
	// Steps keep their definition order, so nodes is always first.
	onb, _ := onboardingPut(t, st, `{"steps":["nodes"]}`)
	if onb == nil {
		t.Fatalf("onboarding missing after marking one step")
	}

	if first := onb.Steps[0]; !first.Done || !first.Manual {
		t.Fatalf("first step = %+v, want done and manual", first)
	}

	// Marking every remaining step empties the checklist.
	onb, _ = onboardingPut(t, st, `{"steps":["storages","bridges","images","profiles","users","policy"]}`)
	if onb != nil {
		t.Fatalf("onboarding = %+v, want nil after marking all done", onb)
	}

	if dash := onboardingDashboard(t, st); dash.Onboarding != nil {
		t.Fatalf("onboarding = %+v, want nil after marking all done", dash.Onboarding)
	}
}
