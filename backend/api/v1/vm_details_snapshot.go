package apiv1

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"

	"pvmss/proxmox"
)

// GetVMSnapshots handles GET /api/v1/vms/:id/snapshots
func (h *VMDetailsHandler) GetVMSnapshots(w http.ResponseWriter, r *http.Request) {
	vmid, ok := requireVMID(w, r)
	if !ok {
		return
	}
	if h.isOffline() {
		errOffline(w)
		return
	}

	username := usernameFromCtx(r)
	isAdmin := isAdminFromCtx(r)

	client, err := restyClient()
	if err != nil {
		writeAppError(w, err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	if !ownsVM(ctx, client, username, isAdmin, vmid) {
		writeError(w, http.StatusNotFound, "not_found", "VM not found")
		return
	}

	node, err := resolveNode(ctx, client, vmid)
	if err != nil || node == "" {
		writeError(w, http.StatusNotFound, "not_found", "VM not found")
		return
	}

	vmidStr := strconv.Itoa(vmid)
	snaps, err := proxmox.GetVMSnapshotsResty(ctx, client, node, vmidStr)
	if err != nil {
		writeAppError(w, err)
		return
	}

	settings := h.state.GetSettings()
	maxSnapshots := settings.Limits.MaxSnapshots
	if maxSnapshots == 0 {
		maxSnapshots = 5
	}

	result := make([]SnapshotResponse, 0, len(snaps))
	for _, s := range snaps {
		result = append(result, SnapshotResponse{
			Name:        s.Name,
			Description: s.Description,
			Snaptime:    s.Snaptime,
			Vmstate:     s.Vmstate,
			Parent:      s.Parent,
			Current:     s.Name == "current",
		})
	}

	writeJSON(w, SnapshotListResponse{Snapshots: result, MaxAllowed: maxSnapshots})
}

// CreateSnapshot handles POST /api/v1/vms/:id/snapshots
func (h *VMDetailsHandler) CreateSnapshot(w http.ResponseWriter, r *http.Request) {
	vmid, ok := requireVMID(w, r)
	if !ok {
		return
	}

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Vmstate     bool   `json:"vmstate"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if !proxmox.IsValidSnapshotName(req.Name) {
		errBadRequest(w, "invalid snapshot name: use only letters, numbers, hyphens and underscores (max 40 chars)")
		return
	}

	username := usernameFromCtx(r)
	isAdmin := isAdminFromCtx(r)

	client, err := restyClient()
	if err != nil {
		writeAppError(w, err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if !ownsVM(ctx, client, username, isAdmin, vmid) {
		writeError(w, http.StatusNotFound, "not_found", "VM not found")
		return
	}

	node, err := resolveNode(ctx, client, vmid)
	if err != nil || node == "" {
		writeError(w, http.StatusNotFound, "not_found", "VM not found")
		return
	}

	vmidStr := strconv.Itoa(vmid)
	if req.Vmstate {
		current, err := proxmox.GetVMCurrentResty(ctx, client, node, vmid)
		if err != nil {
			writeAppError(w, err)
			return
		}
		if current.Status != "running" {
			writeError(w, http.StatusBadRequest, "vm_not_running", "RAM state snapshots can only be created while the VM is running")
			return
		}
		cfg, err := proxmox.GetVMConfigResty(ctx, client, node, vmid)
		if err != nil {
			writeAppError(w, err)
			return
		}
		if !supportsSnapshotVMState(cfg) {
			writeError(w, http.StatusBadRequest, "storage_not_supported", "The underlying storage does not support saving RAM state. Try again without this option.")
			return
		}
	}

	// Enforce snapshot limit
	snaps, err := proxmox.GetVMSnapshotsResty(ctx, client, node, vmidStr)
	if err != nil {
		writeAppError(w, err)
		return
	}
	settings := h.state.GetSettings()
	maxSnapshots := settings.Limits.MaxSnapshots
	if maxSnapshots == 0 {
		maxSnapshots = 5
	}
	actual := 0
	for _, s := range snaps {
		if s.Name != "current" {
			actual++
		}
	}
	if actual >= maxSnapshots {
		writeError(w, http.StatusConflict, "limit_reached", "maximum snapshot limit reached")
		return
	}

	if err := proxmox.CreateVMSnapshotResty(ctx, client, node, vmidStr, proxmox.VMSnapshotConfig{
		Name:        req.Name,
		Description: req.Description,
		Vmstate:     req.Vmstate,
	}); err != nil {
		writeAppError(w, err)
		return
	}

	w.WriteHeader(http.StatusCreated)
	writeJSON(w, map[string]string{"status": "ok"})
}

func supportsSnapshotVMState(cfg map[string]interface{}) bool {
	hasDisks := false
	for key, rawValue := range cfg {
		if !strings.HasPrefix(key, "scsi") && !strings.HasPrefix(key, "virtio") && !strings.HasPrefix(key, "sata") && !strings.HasPrefix(key, "ide") {
			continue
		}
		value, ok := rawValue.(string)
		if !ok || strings.Contains(value, "media=cdrom") {
			continue
		}
		hasDisks = true
		if !diskSupportsVMState(value) {
			return false
		}
	}
	return hasDisks
}

func diskSupportsVMState(disk string) bool {
	normalizedDisk := strings.ToLower(disk)
	if strings.Contains(normalizedDisk, ".qcow2") {
		return true
	}
	storageName := normalizedDisk
	if colonIndex := strings.Index(storageName, ":"); colonIndex >= 0 {
		storageName = storageName[:colonIndex]
	}
	// Check for storage types: match if storage name contains the type as a word component
	// This matches "local-zfs", "ceph-vms", "rbd-storage" but not "cephfs" or "zfs-local"
	storageTypes := []string{"ceph", "rbd", "zfs"}
	for _, storageType := range storageTypes {
		// Check if storage name starts with type, ends with type, or has type surrounded by delimiters
		if strings.HasPrefix(storageName, storageType+"-") ||
			strings.HasSuffix(storageName, "-"+storageType) ||
			strings.Contains(storageName, "-"+storageType+"-") ||
			storageName == storageType {
			return true
		}
	}
	return false
}

// DeleteSnapshot handles DELETE /api/v1/vms/:id/snapshots/:name
func (h *VMDetailsHandler) DeleteSnapshot(w http.ResponseWriter, r *http.Request) {
	vmid, ok := requireVMID(w, r)
	if !ok {
		return
	}
	ps := httprouter.ParamsFromContext(r.Context())
	snapName := ps.ByName("name")
	if snapName == "" || snapName == "current" {
		errBadRequest(w, "invalid snapshot name")
		return
	}

	username := usernameFromCtx(r)
	isAdmin := isAdminFromCtx(r)

	client, err := restyClient()
	if err != nil {
		writeAppError(w, err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if !ownsVM(ctx, client, username, isAdmin, vmid) {
		writeError(w, http.StatusNotFound, "not_found", "VM not found")
		return
	}

	node, err := resolveNode(ctx, client, vmid)
	if err != nil || node == "" {
		writeError(w, http.StatusNotFound, "not_found", "VM not found")
		return
	}

	if err := proxmox.DeleteVMSnapshotResty(ctx, client, node, strconv.Itoa(vmid), snapName); err != nil {
		writeAppError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RollbackSnapshot handles POST /api/v1/vms/:id/snapshots/:name/rollback
func (h *VMDetailsHandler) RollbackSnapshot(w http.ResponseWriter, r *http.Request) {
	vmid, ok := requireVMID(w, r)
	if !ok {
		return
	}
	ps := httprouter.ParamsFromContext(r.Context())
	snapName := ps.ByName("name")
	if snapName == "" || snapName == "current" {
		errBadRequest(w, "invalid snapshot name")
		return
	}

	username := usernameFromCtx(r)
	isAdmin := isAdminFromCtx(r)

	client, err := restyClient()
	if err != nil {
		writeAppError(w, err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if !ownsVM(ctx, client, username, isAdmin, vmid) {
		writeError(w, http.StatusNotFound, "not_found", "VM not found")
		return
	}

	node, err := resolveNode(ctx, client, vmid)
	if err != nil || node == "" {
		writeError(w, http.StatusNotFound, "not_found", "VM not found")
		return
	}

	if err := proxmox.RollbackVMSnapshotResty(ctx, client, node, strconv.Itoa(vmid), snapName); err != nil {
		writeAppError(w, err)
		return
	}
	writeJSON(w, map[string]string{"status": "ok"})
}
