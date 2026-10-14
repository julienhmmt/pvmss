// Package vm implements bulk VM actions. The one function of
// substance here is BulkAction, a loop over the exported
// Action. Later work either wires it to HTTP/frontend or proves,
// by test, a property it already has. Nothing here re-implements
// Resolve()'s tag or pool check, no step reads the Index directly, and no step
// calls cluster.Client directly - every one of those is inside Action(),
// called once per target, unmodified.
package vm

import (
	"context"
	"errors"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/logctx"
	"sync"
	"time"
)

// Per-target status values in BulkTargetResult.Status.
const (
	bulkStatusOK       = "ok"
	bulkStatusAccepted = "accepted"
	bulkStatusError    = "error"
)

// MaxBulkTargets is the upper bound on the number of targets a single bulk
// request may carry. It reuses MaxListPageSize figure
// rather than introducing an independent Configuration field. The handler references this
// constant directly.
const MaxBulkTargets = 100

// BulkTarget identifies one VM inside a bulk request. Cluster is the same
// domain as Resolve() cluster argument and the composite identity
// never a bare vmid, never a client-supplied node. No Node field
// exists anywhere in this shape, matching the single-VM action schema
// (nothing to forge).
type BulkTarget struct {
	Cluster string `json:"cluster"`
	VMID    int    `json:"vmid"`
}

// BulkTargetResult is one entry in a BulkActionResponse. Exactly one entry per
// element of the request's Targets, in the same order. Message is present only
// when Status is "error" - it carries whatever error Action() produced for
// that target (ownership, not-found, or an invalid-state-transition failure
// from the underlying cluster.Client call), verbatim, never re-worded into a
// bulk-specific vocabulary.
type BulkTargetResult struct {
	Cluster string `json:"cluster"`
	VMID    int    `json:"vmid"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
	// Upid is the Proxmox task the action started, when one was started.
	// Status is "accepted" while that task is still running, "ok" once it
	// finished OK, and "error" (Message = exit message) if it failed.
	Upid string `json:"upid,omitempty"`
}

// BulkActionRequest is the POST /api/v1/vms/bulk-action body. Action is one
// value for the whole request - not per-target. Targets is 1 to MaxBulkTargets
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
// bulk handler implements this against the inventory Registry (multi-cluster) or the single
// default projection. BulkAction uses it to dispatch each
// target through its own cluster's projection - the same per-cluster lookup
// Resolve() already performs inside Action(), just lifted one level so the
// loop can span clusters without re-implementing Resolve()'s tag or pool
// check.
type ClusterIndexResolver interface {
	IndexFor(cluster string) (*inventory.Index, error)
}

// ClusterWriterResolver resolves the cluster.Writer for a named cluster -
// the write-side sibling of ClusterIndexResolver. BulkAction's targets may
// span clusters, so a single BulkDeps.Writer cannot vary per target the way
// Resolver already does for the index; WriterResolver closes that gap.
type ClusterWriterResolver interface {
	WriterFor(cluster string) (cluster.Writer, error)
}

// ClusterRefresherResolver resolves the IndexRefresher for a named cluster -
// the refresh-side sibling of ClusterWriterResolver. BulkAction
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
	// StatusReader reads live VM status for idempotent start/stop no-ops.
	// Nil when the caller does not support live-status reads (legacy callers).
	StatusReader cluster.VMStatusReader
	// Force authorizes shutdown to skip ACPI and go directly to stop.
	Force bool
	// TaskWait bounds, in total across every target, how long BulkAction
	// waits for the dispatched Proxmox tasks to settle. Zero skips the wait:
	// started tasks stay "accepted" with their UPID.
	TaskWait time.Duration
}

// BulkAction performs one power transition on every target in targets, in
// array order, and returns one BulkTargetResult per target. It is pure
// orchestration - no logic of its own beyond the loop. Each iteration calls
// the existing, unmodified Action(), which alone performs Resolve()'s
// tag/ownership check, the underlying cluster.Client call, store.RecordAction,
// and the Index invalidation for target.Cluster - all exactly as they
// already happen for a single-VM request.
//
// Action() returns nil → the result entry is {Cluster, VMID, Status: "ok"}.
// Action() returns an error → the result entry is {Cluster, VMID, Status:
// "error", Message: err.Error()} - the error's own message, not a re-derived
// one. This function contains no switch over error kinds and no logic that
// treats a 403 differently from a 502 for the purpose of building the result
// entry.
//
// A target whose cluster has no ready Index produces an "error" entry whose
// Message is the resolver's own error string - the batch never fails as a
// whole.
func BulkAction(
	ctx context.Context,
	deps BulkDeps,
	targets []BulkTarget,
	kind string,
) []BulkTargetResult {
	results := make([]BulkTargetResult, 0, len(targets))
	affectedClusters := make(map[string]struct{})

	var pending []pendingTask

	for i, target := range targets {
		result, task := bulkActionResultFor(ctx, deps, target, kind)
		results = append(results, result)
		affectedClusters[target.Cluster] = struct{}{}

		if task != nil {
			task.index = i
			pending = append(pending, *task)
		}
	}

	results = settleBulkTasks(ctx, deps.TaskWait, results, pending)

	// Refresh once per distinct affected cluster. Not once per
	// target - that was N redundant cluster snapshots. Best-effort: a refresh
	// failure is logged, not returned, so the batch result is unaffected.
	refreshAfterBulk(ctx, deps, affectedClusters)

	return results
}

// refreshAfterBulk refreshes the projection once per distinct cluster that
// was targeted by the bulk action. Uses the per-cluster RefresherResolver
// when available (multi-cluster), falling back to the single Refresher
// (single-cluster mode). Best-effort - errors are swallowed.
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

// pendingTask is a dispatched Proxmox task whose outcome settleBulkTasks
// still has to observe. index is the target's position in the results.
type pendingTask struct {
	index  int
	reader TaskReader
	upid   string
}

// bulkActionResultFor resolves and performs the action for a single target and
// returns its result entry. When the action started a Proxmox task, it also
// returns the task to settle; the entry is then "accepted" until settled.
// Extracted from BulkAction to hold that function under the cognitive-complexity
// ceiling.
func bulkActionResultFor(
	ctx context.Context,
	deps BulkDeps,
	target BulkTarget,
	kind string,
) (BulkTargetResult, *pendingTask) {
	result := BulkTargetResult{Cluster: target.Cluster, VMID: target.VMID}

	index, err := deps.Resolver.IndexFor(target.Cluster)
	if err != nil || index == nil {
		result.Status = bulkStatusError
		if err != nil {
			result.Message = err.Error()
		} else {
			result.Message = "inventory has not been populated yet"
		}

		return result, nil
	}

	targetDeps := deps
	if deps.WriterResolver != nil {
		writer, err := deps.WriterResolver.WriterFor(target.Cluster)
		if err != nil {
			result.Status = bulkStatusError
			result.Message = err.Error()

			return result, nil
		}

		targetDeps.Writer = writer
	}

	upid, err := ActionTask(ctx, targetDeps, index, target.Cluster, target.VMID, kind)
	if err != nil {
		result.Status = bulkStatusError
		result.Message = bulkErrorMessage(ctx, target, err)

		return result, nil
	}

	result.Status = bulkStatusOK
	result.Upid = upid

	if upid == "" {
		return result, nil
	}

	result.Status = bulkStatusAccepted

	// A writer that cannot read task status (no TaskReader) leaves the task
	// unsettled: it stays "accepted" with its UPID, never "ok" or "error".
	reader, ok := targetDeps.Writer.(TaskReader)
	if !ok {
		return result, nil
	}

	return result, &pendingTask{reader: reader, upid: upid}
}

// settleBulkTasks waits for every pending task concurrently under one shared
// budget of wait, so N tasks cost one wait, not N. It returns a new results
// slice: a task that finished OK becomes "ok", one that failed becomes "error"
// with its exit message, and one still unsettled (budget spent, or a failed
// read) stays "accepted" with its UPID.
func settleBulkTasks(ctx context.Context, wait time.Duration, results []BulkTargetResult, pending []pendingTask) []BulkTargetResult {
	if wait <= 0 || len(pending) == 0 {
		return results
	}

	waitCtx, cancel := context.WithTimeout(ctx, wait)
	defer cancel()

	settled := make([]BulkTargetResult, len(pending))

	var group sync.WaitGroup

	for i, task := range pending {
		group.Go(func() {
			settled[i] = settledResult(waitCtx, results[task.index], task)
		})
	}

	group.Wait()

	out := make([]BulkTargetResult, len(results))
	copy(out, results)

	for i, task := range pending {
		out[task.index] = settled[i]
	}

	return out
}

// settledResult maps one task's observed outcome onto its result entry.
func settledResult(ctx context.Context, result BulkTargetResult, task pendingTask) BulkTargetResult {
	status, done, err := AwaitTask(ctx, task.reader, task.upid)
	if err != nil {
		logctx.From(ctx).WarnContext(ctx, "bulk vm task status read failed",
			"component", "vm", "cluster", result.Cluster, "vmid", result.VMID, "error", err)

		return result
	}

	if !done {
		return result
	}

	if status.State == cluster.TaskError {
		result.Status = bulkStatusError
		result.Message = status.ExitMessage

		return result
	}

	result.Status = bulkStatusOK

	return result
}

// bulkErrorMessage keeps PVMSS's own refusals (ownership, invalid transition)
// verbatim but hides a cluster-side failure behind a generic message: the raw
// Proxmox text (paths, internals) goes to the log, not to the user. The batch
// answers 200, so the swallowed error is logged at Warn.
func bulkErrorMessage(ctx context.Context, target BulkTarget, err error) string {
	if !errors.Is(err, cluster.ErrClusterRejected) && !errors.Is(err, cluster.ErrUnreachable) &&
		!errors.Is(err, cluster.ErrTLSVerify) {
		return err.Error()
	}

	logctx.From(ctx).WarnContext(ctx, "bulk vm action failed on the cluster",
		"component", "vm", "cluster", target.Cluster, "vmid", target.VMID, "error", err)

	return "the cluster refused or failed the action"
}
