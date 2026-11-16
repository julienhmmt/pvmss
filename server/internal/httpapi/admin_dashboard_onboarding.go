package httpapi

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"pvmss/server/internal/logctx"
	"pvmss/server/internal/store"
	"slices"
	"strings"
)

// onboardingStepDTO is one first-run setup step, derived from stored state.
// Manual flags a step an admin marked done by hand - a bypass, not proof.
type onboardingStepDTO struct {
	Key        string `json:"key"`
	Done       bool   `json:"done"`
	Required   bool   `json:"required"`
	Count      int    `json:"count"`
	ClusterKey string `json:"clusterKey,omitempty"`
	Manual     bool   `json:"manual,omitempty"`
}

// onboardingDTO is the admin dashboard's setup checklist; nil once complete.
type onboardingDTO struct {
	Steps []onboardingStepDTO `json:"steps"`
}

// onboardingCounter returns one cluster's count for a step. A step is done
// on that cluster when the count is positive.
type onboardingCounter func(ctx context.Context, st *store.Store, row store.ClusterRow) (int, error)

type onboardingStepDef struct {
	key      string
	required bool
	count    onboardingCounter
}

// onboardingSteps is the dependency order: the first undone step is next.
var onboardingSteps = []onboardingStepDef{
	{"nodes", true, rowCount(func(ctx context.Context, st *store.Store, c string) (int, error) {
		rows, err := st.CatalogNodes(ctx, c)
		return len(rows), err
	})},
	{"storages", true, rowCount(func(ctx context.Context, st *store.Store, c string) (int, error) {
		rows, err := st.CatalogStorages(ctx, c)
		return len(rows), err
	})},
	{"bridges", true, rowCount(func(ctx context.Context, st *store.Store, c string) (int, error) {
		rows, err := st.CatalogBridges(ctx, c)
		return len(rows), err
	})},
	{"images", true, rowCount(countImages)},
	{"profiles", true, rowCount(func(ctx context.Context, st *store.Store, c string) (int, error) {
		rows, err := st.CatalogProfiles(ctx, c)
		return len(rows), err
	})},
	{"users", true, countUsers},
	{"policy", false, rowCount(countPolicy)},
}

func rowCount(fn func(ctx context.Context, st *store.Store, cluster string) (int, error)) onboardingCounter {
	return func(ctx context.Context, st *store.Store, row store.ClusterRow) (int, error) {
		return fn(ctx, st, row.Name)
	}
}

// countImages counts templates and ISOs together: either is enough to create a VM.
func countImages(ctx context.Context, st *store.Store, cluster string) (int, error) {
	templates, err := st.CatalogTemplates(ctx, cluster)
	if err != nil {
		return 0, err
	}

	isos, err := st.CatalogISOs(ctx, cluster)

	return len(templates) + len(isos), err
}

// countUsers counts managed pools; SSO on the cluster counts as one, since
// users then sign in without a provisioned account.
func countUsers(ctx context.Context, st *store.Store, row store.ClusterRow) (int, error) {
	pools, err := st.ManagedPools(ctx, row.Name)
	if err != nil {
		return 0, err
	}

	if row.OIDCRealm != "" {
		return len(pools) + 1, nil
	}

	return len(pools), nil
}

// countPolicy counts saved policy edits. Every cluster gets a default policy
// row at creation, so only the audit trail tells "reviewed" from "default".
// shortcut: audit retention may prune the row and bring the (optional) step back.
func countPolicy(ctx context.Context, st *store.Store, cluster string) (int, error) {
	page, err := st.ListAuditLog(ctx, store.AuditFilter{
		Action: "admin.policy.update", TargetType: "policy", TargetID: cluster, Page: 1, PageSize: 1,
	})
	if err != nil {
		return 0, fmt.Errorf("read policy audit: %w", err)
	}

	return page.Total, nil
}

// onboardingSummary returns the setup checklist, or nil once every required
// step is done on at least one cluster. A read failure marks that step done
// (logged): a broken read must not nag the admin about finished work.
func onboardingSummary(ctx context.Context, st *store.Store, log *slog.Logger) *onboardingDTO {
	if st == nil {
		return nil
	}

	clusters, err := st.ListClusters(ctx)
	if err != nil {
		logctx.FromOr(ctx, log).WarnContext(ctx, "failed to read clusters for onboarding", "component", "httpapi", "error", err)
		return nil
	}

	if len(clusters) == 0 {
		return nil
	}

	manual, err := st.OnboardingDoneSteps(ctx)
	if err != nil {
		logctx.FromOr(ctx, log).WarnContext(ctx, "failed to read onboarding overrides", "component", "httpapi", "error", err)

		manual = nil
	}

	steps := make([]onboardingStepDTO, 0, len(onboardingSteps))
	pending := false

	for _, def := range onboardingSteps {
		step := evaluateOnboardingStep(ctx, st, log, def, clusters)
		if _, ok := manual[def.key]; ok && !step.Done {
			step.Done = true
			step.Manual = true
			step.ClusterKey = ""
		}

		if def.required && !step.Done {
			pending = true
		}

		steps = append(steps, step)
	}

	if !pending {
		return nil
	}

	return &onboardingDTO{Steps: steps}
}

func evaluateOnboardingStep(ctx context.Context, st *store.Store, log *slog.Logger, def onboardingStepDef, clusters []store.ClusterRow) onboardingStepDTO {
	step := onboardingStepDTO{Key: def.key, Required: def.required}

	for _, row := range clusters {
		count, err := def.count(ctx, st, row)
		if err != nil {
			logctx.FromOr(ctx, log).WarnContext(ctx, "failed to read onboarding step", "component", "httpapi", "cluster", row.Name, "step", def.key, "error", err)

			count = 1
		}

		step.Count += count
		if count > 0 {
			step.Done = true
		} else if step.ClusterKey == "" {
			step.ClusterKey = row.Name
		}
	}

	if step.Done {
		step.ClusterKey = ""
	}

	return step
}

// onboardingDoneRequest is the body for PUT /api/v1/admin/onboarding: the
// step keys to mark done by hand. Sending every undone key is the
// "mark all done" flow.
type onboardingDoneRequest struct {
	Steps []string `json:"steps"`
}

// onboardingResponseDTO is the PUT reply: the recomputed checklist, null
// once every required step is done.
type onboardingResponseDTO struct {
	Onboarding *onboardingDTO `json:"onboarding"`
}

// ServeOnboardingDone handles PUT /api/v1/admin/onboarding. It lets an admin
// bypass a step the instance cannot satisfy (or chooses not to do), so the
// dashboard checklist can be cleared without touching the catalog.
func (h *AdminOps) ServeOnboardingDone(w http.ResponseWriter, r *http.Request) {
	var req onboardingDoneRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", msgInvalidRequestBody)
		return
	}

	if !validOnboardingKeys(req.Steps) {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "steps must name known onboarding steps")
		return
	}

	actor, ip := h.actorAndIP(r)

	if err := h.store.MarkOnboardingDone(r.Context(), actor.Username, req.Steps); err != nil {
		SetErrorMsg(w, "admin onboarding mark done failed", err)
		writeAdminError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	summary := "onboarding steps marked done: " + strings.Join(req.Steps, ", ")
	_ = h.store.RecordAdminAction(r.Context(), actor.Username, "admin.onboarding.complete", "onboarding", "", detailJSON(summary, nil), ip)

	writeAdminJSON(w, http.StatusOK, onboardingResponseDTO{Onboarding: onboardingSummary(r.Context(), h.store, h.log)})
}

// validOnboardingKeys rejects an empty list and any key outside the defined
// steps, so a typo cannot persist an override nothing will ever read.
func validOnboardingKeys(steps []string) bool {
	if len(steps) == 0 {
		return false
	}

	for _, key := range steps {
		if !slices.ContainsFunc(onboardingSteps, func(def onboardingStepDef) bool { return def.key == key }) {
			return false
		}
	}

	return true
}
