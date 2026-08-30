package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"pvmss/server/internal/cluster"
	"time"
)

// TaskInvalidator rebuilds the inventory projection when a creation task
// completes (FR-018). *inventory.Worker satisfies it; the unguarded
// Refresh is used deliberately — the manual-refresh minimum interval does
// not apply to write-triggered invalidation.
type TaskInvalidator interface {
	Refresh(ctx context.Context) (time.Time, error)
}

// Tasks serves GET /api/v1/tasks/{upid} — a live read of an asynchronous
// cluster task (FR-014). No PVMSS-side task table exists; the state is asked
// of the cluster client on every poll (plan.md research decisions).
//
// UPIDs (Proxmox task IDs) do not embed cluster identity, so a multi-cluster
// deployment cannot tell from a UPID alone which cluster ran it. The caller
// therefore carries the cluster via the ?cluster= query param (the same param
// the rest of the cross-cluster-aware routes use); when clients is non-nil the
// handler resolves that cluster's own Creator per request. Without this, a VM
// created on a non-default cluster would return a UPID that the handler then
// polled against the default cluster's client — silently reporting "not found"
// or foreign state for any non-default-cluster creation.
type Tasks struct {
	auth        *Auth
	creator     cluster.Creator
	clients     cluster.ClientProvider
	invalidator TaskInvalidator
	refreshers  ClusterRefresherResolver
	log         *slog.Logger
}

// NewTasks creates the handler bound to a single cluster.Creator. Use
// NewTasksWithRegistry for multi-cluster deployments so the polled Creator is
// resolved per request from the ?cluster= query param.
func NewTasks(authHandler *Auth, creator cluster.Creator, invalidator TaskInvalidator, log *slog.Logger) *Tasks {
	return &Tasks{auth: authHandler, creator: creator, invalidator: invalidator, log: log}
}

// NewTasksWithRegistry creates the handler with per-request Creator resolution,
// keyed on the request's ?cluster= query param. creator is the default
// cluster's Creator, kept as the fallback for the single-cluster / unit-test
// path (clients == nil), matching every other WithRegistry constructor.
// refreshers resolves the per-cluster invalidator (lifecycle-02); nil keeps
// invalidator as the fallback for every cluster.
func NewTasksWithRegistry(authHandler *Auth, clients cluster.ClientProvider, creator cluster.Creator, invalidator TaskInvalidator, refreshers ClusterRefresherResolver, log *slog.Logger) *Tasks {
	handler := NewTasks(authHandler, creator, invalidator, log)
	handler.clients = clients
	handler.refreshers = refreshers

	return handler
}

type taskStatusDTO struct {
	UPID        string   `json:"upid"`
	State       string   `json:"state"`
	Log         []string `json:"log"`
	ExitMessage string   `json:"exitMessage,omitempty"`
	Warnings    string   `json:"warnings,omitempty"`
}

// ServeHTTP polls one task. When the task is observed in its ok state, the
// inventory index is invalidated so the next VM-list load shows the created
// VM without a manual refresh (FR-018) — not at POST /vms time, when the VM
// does not exist yet.
//
// The endpoint authenticates the caller but does not verify task ownership:
// any authenticated user can poll any UPID. This is accepted for T06 because
// UPIDs are opaque, reveal only creation progress (not VM data), and the
// tray is tab-local. T11+ may tie UPIDs to the actor's pool if needed.
func (h *Tasks) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if _, err := h.auth.Principal(r); err != nil {
		h.writeTaskError(w, http.StatusUnauthorized, "unauthenticated", "authentication required")
		return
	}

	upid := r.PathValue("upid")
	if upid == "" {
		h.writeTaskError(w, http.StatusBadRequest, "invalid_request", "missing task upid")
		return
	}

	clusterName, err := ResolveClusterParam(r, h.clients)
	if err != nil {
		code, message := clusterParamError(err)
		h.writeTaskError(w, http.StatusBadRequest, code, message)

		return
	}

	creator, err := resolveCapability(h.clients, h.creator, clusterName, "Creator")
	if err != nil {
		h.writeTaskError(w, http.StatusNotFound, "cluster_not_found", msgClusterNotFound)

		return
	}

	status, err := creator.TaskStatus(r.Context(), upid)
	if errors.Is(err, cluster.ErrNotFound) {
		h.writeTaskError(w, http.StatusNotFound, "not_found", "unknown task")
		return
	}

	if err != nil {
		h.log.Error("task status read failed", "component", "httpapi", "cluster", clusterName, "error", err)
		// Surface Proxmox's own rejection message when there is one (ADR
		// 0002); transport errors stay generic.
		if code, message, ok := clusterRejectionResponse(err); ok {
			h.writeTaskError(w, http.StatusBadGateway, code, message)
		} else {
			h.writeTaskError(w, http.StatusBadGateway, "cluster_error", "cluster rejected the request")
		}

		return
	}

	if status.State == cluster.TaskOK {
		// Resolve the invalidator per cluster like the Creator above: a task
		// polled with ?cluster=b must invalidate b's projection, not the
		// default cluster's (lifecycle-02 closed this for the write handlers).
		if _, err := h.refresherFor(clusterName).Refresh(r.Context()); err != nil {
			// The task genuinely succeeded; a failed invalidation only delays
			// list visibility until the next automatic cycle — do not fail
			// the poll for it.
			h.log.Error("post-task inventory invalidation failed", "component", "httpapi", "error", err)
		}
	}

	h.writeTaskJSON(w, http.StatusOK, taskStatusDTO{
		UPID:        status.UPID,
		State:       string(status.State),
		Log:         status.Log,
		ExitMessage: status.ExitMessage,
		Warnings:    status.Warnings,
	})
}

// refresherFor resolves the TaskInvalidator for clusterName — the write-side
// sibling of the per-request Creator resolution above (ticket 05). Without
// it, a task polled with ?cluster=b invalidates the default cluster's
// projection instead of b's. A missing resolver or unknown cluster falls back
// to the startup invalidator with a warning — a failed invalidation only
// delays list visibility, never fails the poll.
func (h *Tasks) refresherFor(clusterName string) TaskInvalidator {
	if h.refreshers == nil {
		return h.invalidator
	}

	refresher, err := h.refreshers.RefresherFor(clusterName)
	if err != nil {
		h.log.Warn("task invalidator not found for cluster", "component", "httpapi", "cluster", clusterName, "error", err)

		return h.invalidator
	}

	return refresher
}

func (h *Tasks) writeTaskJSON(w http.ResponseWriter, status int, value any) {
	body, err := json.Marshal(value)
	if err != nil {
		h.log.Error("failed to marshal response", "component", "httpapi", "error", err)
		h.writeTaskError(w, http.StatusInternalServerError, "internal_error", "internal server error")

		return
	}

	if err := writeJSON(w, status, body); err != nil {
		h.log.Error("failed to write response", "component", "httpapi", "error", err)
	}
}

func (h *Tasks) writeTaskError(w http.ResponseWriter, status int, code, message string) {
	if err := writeClusterError(w, status, code, message); err != nil {
		h.log.Error("failed to write error response", "component", "httpapi", "code", code, "error", err)
	}
}
