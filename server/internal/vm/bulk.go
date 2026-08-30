// Package vm implements bulk VM actions (T17 — Actions groupées). The one function of
// substance this tranche produces is BulkAction, a loop over T05's exported
// Action. Every later phase either wires it to HTTP/frontend (US1) or proves,
// by test, a property it already has (US2, US3). No step here re-implements
// Resolve()'s tag or pool check, no step reads the Index directly, and no step
// calls cluster.Client directly — every one of those is inside Action(),
// called once per target, unmodified from T05.
package vm

import (
	"context"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
)

// MaxBulkTargets is the upper bound on the number of targets a single bulk
// request may carry (FR-004, SC-003). It reuses T04's MaxListPageSize figure
// (100) rather than introducing an independent Configuration field — see
// spec.md Assumptions. The handler references this constant directly.
const MaxBulkTargets = 100

// BulkTarget identifies one VM inside a bulk request. Cluster is the same
// domain as T05's Resolve() cluster argument and T15's composite identity —
// never a bare vmid, never a client-supplied node (spec FR-001). No Node field
// exists anywhere in this shape, matching T05's own single-VM action schema
// (nothing to forge).
type BulkTarget struct {
	Cluster string `json:"cluster"`
	VMID    int    `json:"vmid"`
}

// BulkTargetResult is one entry in a BulkActionResponse. Exactly one entry per
// element of the request's Targets, in the same order. Message is present only
// when Status is "error" — it carries whatever error Action() produced for
// that target (ownership, not-found, or an invalid-state-transition failure
// from the underlying cluster.Client call), verbatim, never re-worded into a
// bulk-specific vocabulary.
type BulkTargetResult struct {
	Cluster string `json:"cluster"`
	VMID    int    `json:"vmid"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// BulkActionRequest is the POST /api/v1/vms/bulk-action body. Action is one
// value for the whole request — not per-target. Targets is 1 to MaxBulkTargets
// inclusive; 0 or over the ceiling is rejected before any target is touched.
type BulkActionRequest struct {
	Action  string       `json:"action"`
	Targets []BulkTarget `json:"targets"`
}

// BulkActionResponse is the 200 body. Results has exactly one entry per
// element of the request's Targets, same order, never absent, never empty for
// a request that passed the whole-request validations.
type BulkActionResponse struct {
	Results []BulkTargetResult `json:"results"`
}

// ClusterIndexResolver resolves the current Index for a named cluster. The
// bulk handler implements this against the inventory Registry (multi-cluster,
// T15) or the single default projection. BulkAction uses it to dispatch each
// target through its own cluster's projection — the same per-cluster lookup
// Resolve() already performs inside Action(), just lifted one level so the
// loop can span clusters without re-implementing Resolve()'s tag or pool
// check.
type ClusterIndexResolver interface {
	IndexFor(cluster string) (*inventory.Index, error)
}

// ClusterWriterResolver resolves the cluster.Writer for a named cluster —
// the write-side sibling of ClusterIndexResolver. BulkAction's targets may
// span clusters, so a single BulkDeps.Writer cannot vary per target the way
// Resolver already does for the index; WriterResolver closes that gap.
type ClusterWriterResolver interface {
	WriterFor(cluster string) (cluster.Writer, error)
}

// ClusterRefresherResolver resolves the IndexRefresher for a named cluster —
// the refresh-side sibling of ClusterWriterResolver (ticket 09). BulkAction
// refreshes once per distinct affected cluster after the loop, not once per
// target.
type ClusterRefresherResolver interface {
	RefresherFor(cluster string) (IndexRefresher, error)
}

// BulkDeps groups the collaborators BulkAction (and the per-target Action it
// loops over) need beyond the per-request arguments (ctx, targets, kind).
// Bundling them keeps BulkAction's and Action's parameter counts under
// go:S107's ceiling without losing any dependency. Resolver and
// WriterResolver are used only by BulkAction, to vary the index and writer
// per target; Action ignores them and uses Writer directly (its caller
// already resolved the one clusterName it needs before constructing deps).
type BulkDeps struct {
	Resolver          ClusterIndexResolver
	WriterResolver    ClusterWriterResolver
	RefresherResolver ClusterRefresherResolver
	Actor             auth.Identity
	Writer            cluster.Writer
	Audit             AuditRecorder
	Refresher         IndexRefresher
	// StatusReader reads live VM status for shutdown escalation (ticket 05).
	// Nil when the caller does not support escalation (legacy callers).
	StatusReader cluster.VMStatusReader
	// Force authorizes shutdown to skip ACPI and go directly to stop (ticket 05).
	Force bool
}

// BulkAction performs one power transition on every target in targets, in
// array order, and returns one BulkTargetResult per target. It is pure
// orchestration — no logic of its own beyond the loop. Each iteration calls
// T05's existing, unmodified Action(), which alone performs Resolve()'s
// tag/ownership check, the underlying cluster.Client call, store.RecordAction,
// and the T03/T15 Index invalidation for target.Cluster — all exactly as they
// already happen for a single-VM request.
//
// Action() returns nil → the result entry is {Cluster, VMID, Status: "ok"}.
// Action() returns an error → the result entry is {Cluster, VMID, Status:
// "error", Message: err.Error()} — the error's own message, not a re-derived
// one. This function contains no switch over error kinds and no logic that
// treats a 403 differently from a 502 for the purpose of building the result
// entry.
//
// A target whose cluster has no ready Index produces an "error" entry whose
// Message is the resolver's own error string — the batch never fails as a
// whole (spec FR-005).
func BulkAction(
	ctx context.Context,
	deps BulkDeps,
	targets []BulkTarget,
	kind string,
) []BulkTargetResult {
	results := make([]BulkTargetResult, 0, len(targets))
	affectedClusters := make(map[string]struct{})

	for _, target := range targets {
		results = append(results, bulkActionResultFor(ctx, deps, target, kind))
		affectedClusters[target.Cluster] = struct{}{}
	}

	// Refresh once per distinct affected cluster (ticket 09). Not once per
	// target — that was N redundant cluster snapshots. Best-effort: a refresh
	// failure is logged, not returned, so the batch result is unaffected.
	refreshAfterBulk(ctx, deps, affectedClusters)

	return results
}

// refreshAfterBulk refreshes the projection once per distinct cluster that
// was targeted by the bulk action. Uses the per-cluster RefresherResolver
// when available (multi-cluster), falling back to the single Refresher
// (single-cluster mode). Best-effort — errors are swallowed.
func refreshAfterBulk(ctx context.Context, deps BulkDeps, clusters map[string]struct{}) {
	for clusterName := range clusters {
		refresher := deps.Refresher
		if deps.RefresherResolver != nil {
			if r, err := deps.RefresherResolver.RefresherFor(clusterName); err == nil {
				refresher = r
			}
		}

		if refresher == nil {
			continue
		}

		_, _ = refresher.Refresh(ctx)
	}
}

// bulkActionResultFor resolves and performs the action for a single target and
// returns its result entry. Extracted from BulkAction to hold that function
// under the cognitive-complexity ceiling; logic is identical.
func bulkActionResultFor(
	ctx context.Context,
	deps BulkDeps,
	target BulkTarget,
	kind string,
) BulkTargetResult {
	result := BulkTargetResult{Cluster: target.Cluster, VMID: target.VMID}

	index, err := deps.Resolver.IndexFor(target.Cluster)
	if err != nil || index == nil {
		result.Status = "error"
		if err != nil {
			result.Message = err.Error()
		} else {
			result.Message = "inventory has not been populated yet"
		}

		return result
	}

	targetDeps := deps
	if deps.WriterResolver != nil {
		writer, err := deps.WriterResolver.WriterFor(target.Cluster)
		if err != nil {
			result.Status = "error"
			result.Message = err.Error()

			return result
		}

		targetDeps.Writer = writer
	}

	if err := Action(ctx, targetDeps, index, target.Cluster, target.VMID, kind); err != nil {
		result.Status = "error"
		result.Message = err.Error()
	} else {
		result.Status = "ok"
	}

	return result
}
