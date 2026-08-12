// Package vm: bulk.go implements T17 — Actions groupées. The one function of
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
	resolver ClusterIndexResolver,
	actor auth.Identity,
	targets []BulkTarget,
	kind string,
	writer cluster.Writer,
	audit AuditRecorder,
	refresher IndexRefresher,
) []BulkTargetResult {
	results := make([]BulkTargetResult, 0, len(targets))
	for _, target := range targets {
		result := BulkTargetResult{Cluster: target.Cluster, VMID: target.VMID}

		index, err := resolver.IndexFor(target.Cluster)
		if err != nil || index == nil {
			result.Status = "error"
			if err != nil {
				result.Message = err.Error()
			} else {
				result.Message = "inventory has not been populated yet"
			}
			results = append(results, result)
			continue
		}

		if err := Action(ctx, index, actor, target.Cluster, target.VMID, kind, writer, audit, refresher); err != nil {
			result.Status = "error"
			result.Message = err.Error()
		} else {
			result.Status = "ok"
		}

		results = append(results, result)
	}

	return results
}
