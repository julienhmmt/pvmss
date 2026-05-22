package apiv1

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"

	"pvmss/logger"
	"pvmss/proxmox"
	"pvmss/state"
)

// VMDiskHandler handles disk management endpoints.
type VMDiskHandler struct {
	state state.StateManager
}

// MakeVMDiskHandler creates a new VMDiskHandler.
func MakeVMDiskHandler(s state.StateManager) *VMDiskHandler {
	return &VMDiskHandler{state: s}
}

func (h *VMDiskHandler) isOffline() bool {
	return h.state != nil && h.state.IsOfflineMode()
}

// diskBusOrder defines the canonical iteration order for disk bus types.
var diskBusOrder = []string{"virtio", "scsi", "sata", "ide"}

// maxDiskSlotRetries is the maximum number of retry attempts for disk slot allocation
// when handling race conditions during concurrent disk additions.
const maxDiskSlotRetries = 3

// validDiskBusTypes is the set of accepted disk bus types (for O(1) lookups).
var validDiskBusTypes = map[string]bool{
	"virtio": true,
	"scsi":   true,
	"sata":   true,
	"ide":    true,
}

// countTotalDisks counts all non-CDROM disks across all bus types in a VM config.
// CDROM entries (media=cdrom) and IDE ISO entries are excluded.
func countTotalDisks(cfg map[string]interface{}) int {
	count := 0
	for _, bus := range diskBusOrder {
		maxDisks := state.GetMaxDisksForBus(bus)
		for i := 0; i < maxDisks; i++ {
			key := bus + strconv.Itoa(i)
			val, exists := cfg[key]
			if !exists {
				continue
			}
			s, ok := val.(string)
			if !ok || s == "" {
				continue
			}
			// Skip CDROM drives
			if strings.Contains(s, "media=cdrom") {
				continue
			}
			// Skip ISO files on IDE bus (e.g., "local:ubuntu.iso")
			if bus == "ide" {
				parts := strings.SplitN(s, ",", 2)
				if len(parts) > 0 && strings.HasSuffix(parts[0], ".iso") {
					continue
				}
			}
			count++
		}
	}
	return count
}

// blockStorages are storage types that require raw disk format.
var blockStorages = map[string]bool{
	"lvm":     true,
	"lvmthin": true,
	"zfs":     true,
	"ceph":    true,
	"iscsi":   true,
}

// getDiskFormat returns the appropriate disk format based on storage type.
// Block-based storages (lvm, lvmthin, zfs, ceph, iscsi) use raw.
// File-based storages (dir, nfs, cifs) use qcow2.
// If storage type is unknown, attempts to infer from storage name patterns.
func getDiskFormat(storageType string) string {
	if blockStorages[storageType] {
		return "raw"
	}
	return "qcow2"
}

// inferDiskFormatFromName attempts to infer disk format from storage name patterns
// when storage type information is unavailable. This is a fallback heuristic.
func inferDiskFormatFromName(storageName string) string {
	lowerName := strings.ToLower(storageName)
	// Common patterns for block-based storages
	if strings.Contains(lowerName, "lvm") || strings.Contains(lowerName, "zfs") ||
		strings.Contains(lowerName, "ceph") || strings.Contains(lowerName, "rbd") {
		return "raw"
	}
	// Default to qcow2 for unknown storage types
	return "qcow2"
}

// isBootDisk checks if the given disk ID is the boot disk based on VM config.
// The boot disk is any disk listed in the boot order. When no explicit boot
// order is set, all index-0 disks (virtio0, scsi0, sata0, ide0) are
// considered potential boot disks and protected from deletion.
func isBootDisk(cfg map[string]interface{}, diskID string) bool {
	// Check boot order configuration
	// Proxmox format: "order=virtio0;ide2" — a single string with semicolons
	if bootOrder, ok := cfg["boot"].(string); ok && bootOrder != "" {
		// Strip optional "order=" prefix and split by semicolons
		orderStr := strings.TrimPrefix(bootOrder, "order=")
		for _, d := range strings.Split(orderStr, ";") {
			if strings.TrimSpace(d) == diskID {
				return true
			}
		}
		// Explicit boot order present but this disk is not in it → not a boot disk
		return false
	}

	// No explicit boot order: protect all index-0 disks as potential boot devices.
	// Proxmox defaults to booting from the first available device when no
	// order is configured, so any bus index 0 could be the boot disk.
	for _, bus := range diskBusOrder {
		if diskID == bus+"0" {
			return true
		}
	}
	return false
}

// nextDiskSlot finds the lowest unused index for a given bus type in a VM config.
func nextDiskSlot(cfg map[string]interface{}, bus string) (int, error) {
	maxDisks := state.GetMaxDisksForBus(bus)
	for i := 0; i < maxDisks; i++ {
		key := bus + strconv.Itoa(i)
		if _, exists := cfg[key]; !exists {
			return i, nil
		}
	}
	return -1, fmt.Errorf("no available disk slots for bus %s (max %d)", bus, maxDisks)
}

// isDiskIDValid returns true if diskID is a valid bus+index pair (e.g., "scsi0", "virtio1").
func isDiskIDValid(diskID string) bool {
	for _, bus := range diskBusOrder {
		if strings.HasPrefix(diskID, bus) {
			rest := strings.TrimPrefix(diskID, bus)
			if _, err := strconv.Atoi(rest); err == nil {
				return true
			}
		}
	}
	return false
}

// isCDROMEntry returns true if the disk config string represents a CDROM drive.
func isCDROMEntry(diskConfig string) bool {
	return strings.Contains(diskConfig, "media=cdrom")
}

// AddDisk handles POST /api/v1/vms/:id/disks
// Creates a new disk on the VM by allocating the next available slot for the chosen bus.
func (h *VMDiskHandler) AddDisk(w http.ResponseWriter, r *http.Request) {
	vmid, ok := requireVMID(w, r)
	if !ok {
		return
	}

	if h.isOffline() {
		errOffline(w)
		return
	}

	var req struct {
		Storage string `json:"storage"`
		SizeGB  int    `json:"size_gb"`
		Bus     string `json:"bus"`
	}
	if !decodeBody(w, r, &req) {
		return
	}

	req.Bus = strings.ToLower(strings.TrimSpace(req.Bus))
	req.Storage = strings.TrimSpace(req.Storage)

	if !validDiskBusTypes[req.Bus] {
		errBadRequest(w, "invalid bus type: must be virtio, scsi, sata, or ide")
		return
	}
	if req.Storage == "" {
		errBadRequest(w, "storage is required")
		return
	}

	settings := h.state.GetSettings()

	// Validate storage against allowed list.
	// Handles both "node:storage" and plain "storage" formats in EnabledStorages.
	// Empty EnabledStorages list means any storage is allowed (no restriction).
	if len(settings.EnabledStorages) > 0 {
		storageAllowed := false
		for _, enabledStorage := range settings.EnabledStorages {
			parts := strings.SplitN(enabledStorage, ":", 2)
			storageName := enabledStorage
			if len(parts) == 2 {
				storageName = parts[1]
			}
			if storageName == req.Storage {
				storageAllowed = true
				break
			}
		}
		if !storageAllowed {
			errBadRequest(w, "storage not allowed")
			return
		}
	}

	// Validate size
	if req.SizeGB <= 0 {
		errBadRequest(w, "disk size is required and must be positive")
		return
	}
	diskMin := settings.Limits.VM.Disk.Min
	diskMax := settings.Limits.VM.Disk.Max
	if diskMin < 1 {
		diskMin = 1
	}
	if req.SizeGB < diskMin || req.SizeGB > diskMax {
		errBadRequest(w, fmt.Sprintf("disk size must be between %d and %d GB", diskMin, diskMax))
		return
	}

	username := usernameFromCtx(r)
	isAdmin := isAdminFromCtx(r)

	client, err := restyClient()
	if err != nil {
		writeAppError(w, err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
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

	// Check VM status - disk operations require VM to be stopped
	current, err := proxmox.GetVMCurrentResty(ctx, client, node, vmid)
	if err != nil {
		writeAppError(w, err)
		return
	}
	if current.Status == "running" {
		writeError(w, http.StatusConflict, "vm_running", "Disk operations require the VM to be stopped")
		return
	}

	// Verify storage exists on this node and fetch storage type for format detection
	nodeStorages, err := proxmox.GetNodeStoragesResty(ctx, client, node)
	if err != nil {
		logger.Get().Warn().Err(err).Str("node", node).Msg("Failed to fetch node storages for validation")
		// Continue anyway - Proxmox will validate at API level
	} else {
		storageExistsOnNode := false
		for _, s := range nodeStorages {
			if s.Storage == req.Storage {
				storageExistsOnNode = true
				break
			}
		}
		if !storageExistsOnNode {
			writeError(w, http.StatusBadRequest, "storage_not_on_node", fmt.Sprintf("Storage '%s' does not exist on node '%s'", req.Storage, node))
			return
		}
	}

	cfg, err := proxmox.GetVMConfigResty(ctx, client, node, vmid)
	if err != nil {
		writeAppError(w, err)
		return
	}

	// Validate total disk count against MaxDiskPerVM
	totalDisks := countTotalDisks(cfg)
	if totalDisks >= settings.MaxDiskPerVM {
		writeError(w, http.StatusConflict, "max_disks_exceeded", fmt.Sprintf("Maximum number of disks (%d) reached for this VM", settings.MaxDiskPerVM))
		return
	}

	slot, err := nextDiskSlot(cfg, req.Bus)
	if err != nil {
		writeError(w, http.StatusConflict, "no_slot", err.Error())
		return
	}

	diskKey := req.Bus + strconv.Itoa(slot)

	// Determine disk format based on storage type.
	// Use the already-fetched nodeStorages to avoid duplicate API call.
	var diskVal string
	if nodeStorages != nil {
		// Find the storage type
		storageType := ""
		storageFound := false
		for _, s := range nodeStorages {
			if s.Storage == req.Storage {
				storageType = s.Type
				storageFound = true
				break
			}
		}
		if !storageFound {
			logger.Get().Warn().Str("storage", req.Storage).Str("node", node).Msg("Storage not found in node storage list, omitting format param")
			diskVal = fmt.Sprintf("%s:%d", req.Storage, req.SizeGB)
		} else {
			diskFormat := getDiskFormat(storageType)
			diskVal = fmt.Sprintf("%s:%d,format=%s", req.Storage, req.SizeGB, diskFormat)
		}
	} else {
		// Storage fetch failed earlier - infer format from storage name pattern
		// This provides a safer default than letting Proxmox choose
		diskFormat := inferDiskFormatFromName(req.Storage)
		diskVal = fmt.Sprintf("%s:%d,format=%s", req.Storage, req.SizeGB, diskFormat)
	}

	// Retry on conflict (race condition handling)
	// Note: diskVal is not recalculated on retry because it depends only on
	// req.Storage, req.SizeGB, and storage type — none of which change between retries.
	for retry := 0; retry < maxDiskSlotRetries; retry++ {
		if err := proxmox.UpdateVMConfigResty(ctx, client, node, vmid, map[string]string{
			diskKey: diskVal,
		}); err != nil {
			// Check if error is due to slot already taken (conflict)
			if strings.Contains(err.Error(), "parameter verification failed") || strings.Contains(err.Error(), "already exists") {
				if retry < maxDiskSlotRetries-1 {
					// Re-fetch config and try next slot
					cfg, err = proxmox.GetVMConfigResty(ctx, client, node, vmid)
					if err != nil {
						writeAppError(w, err)
						return
					}
					slot, err = nextDiskSlot(cfg, req.Bus)
					if err != nil {
						writeError(w, http.StatusConflict, "no_slot", err.Error())
						return
					}
					diskKey = req.Bus + strconv.Itoa(slot)
					continue
				}
			}
			writeAppError(w, err)
			return
		}
		// Success
		break
	}

	// UpdateVMConfigResty already invalidates the VM cache

	logger.Get().Info().Int("vmid", vmid).Str("disk", diskKey).Str("storage", req.Storage).Int("size_gb", req.SizeGB).Msg("Disk added")
	writeJSONStatus(w, http.StatusCreated, map[string]string{"status": "ok", "disk": diskKey})
}

// ResizeDisk handles PUT /api/v1/vms/:id/disks/:diskId/resize
// Increases a disk's size by the specified amount in GB (only increases are supported by Proxmox).
func (h *VMDiskHandler) ResizeDisk(w http.ResponseWriter, r *http.Request) {
	vmid, ok := requireVMID(w, r)
	if !ok {
		return
	}

	ps := httprouter.ParamsFromContext(r.Context())
	diskID := strings.ToLower(strings.TrimSpace(ps.ByName("diskId")))
	if !isDiskIDValid(diskID) {
		errBadRequest(w, "invalid disk id")
		return
	}

	if h.isOffline() {
		errOffline(w)
		return
	}

	var req struct {
		AddGB int `json:"add_gb"`
	}
	if !decodeBody(w, r, &req) {
		return
	}

	if req.AddGB <= 0 {
		errBadRequest(w, "add_gb must be a positive integer (Proxmox does not support shrinking disks)")
		return
	}

	settings := h.state.GetSettings()

	diskMax := settings.Limits.VM.Disk.Max
	if diskMax < 1 {
		diskMax = 2000 // Safe fallback if not configured
	}
	if req.AddGB > diskMax {
		errBadRequest(w, fmt.Sprintf("Cannot add more than %d GB at once", diskMax))
		return
	}

	username := usernameFromCtx(r)
	isAdmin := isAdminFromCtx(r)

	client, err := restyClient()
	if err != nil {
		writeAppError(w, err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
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

	// Note: Proxmox supports disk resize on running VMs, so no running-state check here.

	// Get current disk size to validate final size
	cfg, err := proxmox.GetVMConfigResty(ctx, client, node, vmid)
	if err != nil {
		writeAppError(w, err)
		return
	}

	// Verify disk exists
	diskConfig, exists := cfg[diskID].(string)
	if !exists {
		writeError(w, http.StatusNotFound, "disk_not_found", "Disk not found in VM configuration")
		return
	}

	// Reject CDROM drives — they cannot be resized
	if isCDROMEntry(diskConfig) {
		writeError(w, http.StatusBadRequest, "cdrom", "Cannot resize a CDROM drive")
		return
	}

	// Parse current disk size from the "size=NG" parameter.
	// Proxmox config format: "local-lvm:vm-100-disk-0,size=10G,format=raw"
	// The "storage:size" shortcut only applies at creation time, not in the stored config.
	var currentSizeGB int
	for _, part := range strings.Split(diskConfig, ",") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "size=") {
			s := strings.TrimPrefix(part, "size=")
			if strings.HasSuffix(s, "G") {
				if n, parseErr := strconv.Atoi(strings.TrimSuffix(s, "G")); parseErr == nil {
					currentSizeGB = n
				}
			} else if strings.HasSuffix(s, "T") {
				if n, parseErr := strconv.Atoi(strings.TrimSuffix(s, "T")); parseErr == nil {
					currentSizeGB = n * 1024
				}
			} else if strings.HasSuffix(s, "M") {
				// Parse megabyte values and convert to GB
				// Round up sub-GB sizes to 1 GB minimum to prevent zero-size disks
				// This is a reasonable fallback for edge cases where size is specified in MB
				if n, parseErr := strconv.Atoi(strings.TrimSuffix(s, "M")); parseErr == nil {
					currentSizeGB = n / 1024
					if currentSizeGB == 0 {
						currentSizeGB = 1 // Round up sub-GB sizes to 1 GB minimum
					}
				}
			}
			break
		}
	}

	if currentSizeGB <= 0 {
		// Fallback: Try to get disk size from storage content API
		// Extract storage name from disk config (format: "storage:volume-id" or "storage:volume-id,size=...")
		storageName := ""
		if idx := strings.Index(diskConfig, ":"); idx > 0 {
			storageName = diskConfig[:idx]
		}
		if storageName != "" {
			storageContent, err := proxmox.GetAllStorageContentResty(ctx, client, node, storageName)
			if err == nil {
				// Look for the disk volume in the storage content
				diskVolID := strings.Split(diskConfig, ",")[0] // Get the volid part before any commas
				for _, item := range storageContent {
					if item.VolID == diskVolID {
						// Convert bytes to GB
						currentSizeGB = int(item.Size / (1024 * 1024 * 1024))
						if currentSizeGB == 0 {
							currentSizeGB = 1 // Minimum 1 GB
						}
						logger.Get().Debug().
							Str("disk", diskID).
							Str("storage", storageName).
							Int("size_gb", currentSizeGB).
							Msg("Retrieved disk size from storage content API")
						break
					}
				}
			}
		}
		if currentSizeGB <= 0 {
			writeError(w, http.StatusBadRequest, "unknown_size", "Could not determine current disk size from config or storage")
			return
		}
	}

	// Validate final size against maximum disk limit
	finalSizeGB := currentSizeGB + req.AddGB
	if finalSizeGB > diskMax {
		writeError(w, http.StatusBadRequest, "size_exceeds_max", fmt.Sprintf("Final disk size (%d GB) exceeds maximum allowed size (%d GB)", finalSizeGB, diskMax))
		return
	}

	// Log the resize for observability; Proxmox enforces storage capacity limits.
	logger.Get().Debug().
		Int("vmid", vmid).Str("disk", diskID).
		Int("current_gb", currentSizeGB).Int("add_gb", req.AddGB).
		Msg("ResizeDisk: validated, sending to Proxmox")

	sizeIncrement := fmt.Sprintf("+%dG", req.AddGB)
	if err := proxmox.ResizeVMDiskResty(ctx, client, node, vmid, diskID, sizeIncrement); err != nil {
		writeAppError(w, err)
		return
	}

	// ResizeVMDiskResty does not call UpdateVMConfigResty, so we must invalidate manually
	proxmox.InvalidateVMCache(node)

	logger.Get().Info().Int("vmid", vmid).Str("disk", diskID).Int("add_gb", req.AddGB).Msg("Disk resized")
	w.WriteHeader(http.StatusNoContent)
}

// DeleteDisk handles DELETE /api/v1/vms/:id/disks/:diskId
// Detaches (and marks unused) the specified disk from the VM config.
// Note: this does not immediately destroy the underlying storage volume.
func (h *VMDiskHandler) DeleteDisk(w http.ResponseWriter, r *http.Request) {
	vmid, ok := requireVMID(w, r)
	if !ok {
		return
	}

	ps := httprouter.ParamsFromContext(r.Context())
	diskID := strings.ToLower(strings.TrimSpace(ps.ByName("diskId")))
	if !isDiskIDValid(diskID) {
		errBadRequest(w, "invalid disk id")
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

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
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

	// Check VM status - disk operations require VM to be stopped
	current, err := proxmox.GetVMCurrentResty(ctx, client, node, vmid)
	if err != nil {
		writeAppError(w, err)
		return
	}
	if current.Status == "running" {
		writeError(w, http.StatusConflict, "vm_running", "Disk operations require the VM to be stopped")
		return
	}

	// Get VM config to verify disk exists and check if it's the boot disk
	cfg, err := proxmox.GetVMConfigResty(ctx, client, node, vmid)
	if err != nil {
		writeAppError(w, err)
		return
	}

	// Verify disk exists and is not a CDROM
	diskConfig, diskExists := cfg[diskID].(string)
	if !diskExists {
		writeError(w, http.StatusNotFound, "disk_not_found", "Disk not found in VM configuration")
		return
	}
	if isCDROMEntry(diskConfig) {
		writeError(w, http.StatusBadRequest, "cdrom", "Cannot detach a CDROM drive")
		return
	}

	// Check if this is the boot disk
	if isBootDisk(cfg, diskID) {
		writeError(w, http.StatusForbidden, "boot_disk", "Cannot delete the boot disk")
		return
	}

	if err := proxmox.UpdateVMConfigResty(ctx, client, node, vmid, map[string]string{
		"delete": diskID,
	}); err != nil {
		writeAppError(w, err)
		return
	}
	// UpdateVMConfigResty already invalidates the VM cache

	logger.Get().Info().Int("vmid", vmid).Str("disk", diskID).Msg("Disk detached")
	w.WriteHeader(http.StatusNoContent)
}
