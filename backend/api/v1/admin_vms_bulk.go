package apiv1

import (
	"context"
	"net/http"
	"strconv"
	"sync"
	"time"

	"pvmss/constants"
	"pvmss/logger"
	"pvmss/proxmox"
)

const (
	// maxBulkBatch caps the number of VMs a single bulk request may target.
	maxBulkBatch = 50
	// maxBulkConcurrent throttles concurrent Proxmox calls (matches the
	// nodes-aggregator fan-out cap; guards per-token rate limits).
	maxBulkConcurrent = 8
)

// BulkVMActionRequest is the body for POST /api/v1/admin/vms-bulk-action.
type BulkVMActionRequest struct {
	Action string `json:"action"`
	VMIDs  []int  `json:"vmids"`
}

// BulkVMResult reports the outcome for one VM in a bulk request.
type BulkVMResult struct {
	VMID   int    `json:"vmid"`
	OK     bool   `json:"ok"`
	TaskID string `json:"task_id,omitempty"`
	Error  string `json:"error,omitempty"`
}

// BulkVMActionResponse is the always-200 envelope for a bulk action.
type BulkVMActionResponse struct {
	Action    string         `json:"action"`
	Requested int            `json:"requested"`
	Accepted  int            `json:"accepted"`
	Failed    int            `json:"failed"`
	Results   []BulkVMResult `json:"results"`
}

// validateBulkVMIDs de-duplicates and bounds-checks the requested VMIDs,
// preserving first-seen order. It returns a non-empty message on rejection.
func validateBulkVMIDs(ids []int) ([]int, string) {
	seen := make(map[int]bool, len(ids))
	out := make([]int, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return nil, "vmids must be positive integers"
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	if len(out) == 0 {
		return nil, "vmids must contain at least one VM"
	}
	if len(out) > maxBulkBatch {
		return nil, "too many VMs: maximum " + strconv.Itoa(maxBulkBatch) + " per request"
	}
	return out, ""
}

// vmActionFn executes a single VM action, returning the task UPID. Injectable
// so runBulkVMAction can be unit-tested without a live Proxmox endpoint.
type vmActionFn func(ctx context.Context, node, vmid, action string) (string, error)

// runBulkVMAction fans out action across vmids (best-effort, order-preserving),
// resolving each VM's node from nodeByVMID. VMs absent from the map are reported
// as failed without being dispatched. One failure never cancels its peers.
func runBulkVMAction(ctx context.Context, action string, vmids []int, nodeByVMID map[int]string, act vmActionFn) []BulkVMResult {
	results := make([]BulkVMResult, len(vmids))
	sem := make(chan struct{}, maxBulkConcurrent)
	var wg sync.WaitGroup

	for i, vmid := range vmids {
		node, ok := nodeByVMID[vmid]
		if !ok {
			results[i] = BulkVMResult{VMID: vmid, OK: false, Error: "VM not found or not pvmss-tagged"}
			continue
		}
		wg.Add(1)
		go func(i, vmid int, node string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			upid, err := act(ctx, node, strconv.Itoa(vmid), action)
			if err != nil {
				results[i] = BulkVMResult{VMID: vmid, OK: false, Error: "proxmox action failed"}
				return
			}
			results[i] = BulkVMResult{VMID: vmid, OK: true, TaskID: upid}
		}(i, vmid, node)
	}
	wg.Wait()
	return results
}

// BulkVMAction handles POST /api/v1/admin/vms-bulk-action (admin-only).
func (h *AdminVMsAPIHandler) BulkVMAction(w http.ResponseWriter, r *http.Request) {
	if h.state.IsOfflineMode() {
		errOffline(w)
		return
	}

	var req BulkVMActionRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if !allowedActions[req.Action] {
		errBadRequest(w, "action must be one of: start, stop, shutdown, reboot, reset")
		return
	}

	vmids, msg := validateBulkVMIDs(req.VMIDs)
	if msg != "" {
		errBadRequest(w, msg)
		return
	}

	cfg := h.state.GetEnvConfig()
	client, err := proxmox.MakeRestyClientFromEnvConfig(cfg, 30*time.Second)
	if err != nil {
		writeAppError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	// One list call → vmid→node map, enforcing pvmss-tag membership.
	vms, err := proxmox.GetVMsResty(ctx, client)
	if err != nil {
		writeAppError(w, err)
		return
	}
	nodeByVMID := make(map[int]string, len(vms))
	for _, vm := range vms {
		if hasTag(vm.Tags, constants.RequiredTag) {
			nodeByVMID[vm.VMID] = vm.Node
		}
	}

	act := func(ctx context.Context, node, vmid, action string) (string, error) {
		return proxmox.VMActionResty(ctx, client, node, vmid, action)
	}
	results := runBulkVMAction(ctx, req.Action, vmids, nodeByVMID, act)

	accepted := 0
	for _, res := range results {
		if res.OK {
			accepted++
		}
	}
	failed := len(results) - accepted

	logger.Get().Info().
		Str("action", req.Action).
		Str("username", usernameFromCtx(r)).
		Int("requested", len(vmids)).
		Int("accepted", accepted).
		Int("failed", failed).
		Msg("api/v1: bulk VM action dispatched")

	writeJSON(w, BulkVMActionResponse{
		Action:    req.Action,
		Requested: len(vmids),
		Accepted:  accepted,
		Failed:    failed,
		Results:   results,
	})
}
