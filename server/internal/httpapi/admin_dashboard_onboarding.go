package httpapi

import (
	"context"
	"fmt"
	"log/slog"
	"pvmss/server/internal/logctx"
	"pvmss/server/internal/store"
)

// onboardingStepDTO is one first-run setup step, derived from stored state.
type onboardingStepDTO struct {
	Key        string `json:"key"`
	Done       bool   `json:"done"`
	Required   bool   `json:"required"`
	Count      int    `json:"count"`
	ClusterKey string `json:"clusterKey,omitempty"`
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

	steps := make([]onboardingStepDTO, 0, len(onboardingSteps))
	pending := false

	for _, def := range onboardingSteps {
		step := evaluateOnboardingStep(ctx, st, log, def, clusters)
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
