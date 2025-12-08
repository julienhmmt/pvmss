package handlers

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"pvmss/logger"
	"pvmss/proxmox"
	"pvmss/state"

	"github.com/julienschmidt/httprouter"
)

// VMSnapshotsHandler handles snapshot-related operations for VMs
type VMSnapshotsHandler struct {
	stateManager state.StateManager
}

// NewVMSnapshotsHandler creates a new VMSnapshotsHandler
func NewVMSnapshotsHandler(stateManager state.StateManager) *VMSnapshotsHandler {
	return &VMSnapshotsHandler{
		stateManager: stateManager,
	}
}

// CreateVMSnapshotHandler handles the creation of a VM snapshot
func (h *VMSnapshotsHandler) CreateVMSnapshotHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	log := logger.Get()

	if !ValidateMethodAndParseForm(w, r, http.MethodPost) {
		return
	}

	// Get VM details from form
	vmidStr := r.FormValue("vmid")
	node := r.FormValue("node")
	snapshotName := r.FormValue("snapshot_name")
	description := r.FormValue("description")

	if vmidStr == "" || node == "" || snapshotName == "" {
		log.Error().Msg("Missing required parameters for snapshot creation")
		http.Redirect(w, r, "/vm/details?vmid="+vmidStr+"&node="+node+"&error=missing_parameters", http.StatusFound)
		return
	}

	vmidInt, err := strconv.Atoi(vmidStr)
	if err != nil {
		log.Error().Err(err).Msg("Invalid VM ID")
		http.Redirect(w, r, "/vm/details?vmid="+vmidStr+"&node="+node+"&error=invalid_vmid", http.StatusFound)
		return
	}

	// Validate snapshot name
	if !proxmox.IsValidSnapshotName(snapshotName) {
		log.Error().Str("snapshot_name", snapshotName).Msg("Invalid snapshot name")
		http.Redirect(w, r, "/vm/details?vmid="+vmidStr+"&node="+node+"&error=invalid_snapshot_name", http.StatusFound)
		return
	}

	// Get Proxmox client
	restyClient, err := getDefaultRestyClient()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get Proxmox client")
		http.Redirect(w, r, "/vm/details?vmid="+vmidStr+"&node="+node+"&error=client_error", http.StatusFound)
		return
	}

	// Check current snapshot count to enforce limit
	snapshots, err := proxmox.GetVMSnapshotsResty(r.Context(), restyClient, node, vmidStr)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get current snapshots")
		http.Redirect(w, r, "/vm/details?vmid="+vmidStr+"&node="+node+"&error=fetch_snapshots", http.StatusFound)
		return
	}

	// Get settings to check max snapshots limit
	settings, _, err := state.LoadSettings()
	if err != nil {
		log.Error().Err(err).Msg("Failed to load settings")
		http.Redirect(w, r, "/vm/details?vmid="+vmidStr+"&node="+node+"&error=settings_error", http.StatusFound)
		return
	}

	if len(snapshots) >= settings.MaxSnapshotsPerVM {
		log.Error().
			Int("current_count", len(snapshots)).
			Int("max_allowed", settings.MaxSnapshotsPerVM).
			Msg("Maximum snapshot limit reached")
		http.Redirect(w, r, "/vm/details?vmid="+vmidStr+"&node="+node+"&error=max_snapshots_reached", http.StatusFound)
		return
	}

	// Create snapshot
	snapshotConfig := proxmox.VMSnapshotConfig{
		Name:        snapshotName,
		Description: description,
		Vmstate:     true, // Include VM state for rollback
	}

	err = proxmox.CreateVMSnapshotResty(r.Context(), restyClient, node, vmidStr, snapshotConfig)
	if err != nil {
		log.Error().Err(err).Str("snapshot_name", snapshotName).Msg("Failed to create snapshot")
		http.Redirect(w, r, "/vm/details?vmid="+vmidStr+"&node="+node+"&error=create_failed", http.StatusFound)
		return
	}

	log.Info().
		Int("vmid", vmidInt).
		Str("node", node).
		Str("snapshot_name", snapshotName).
		Msg("VM snapshot created successfully")

	// Force refresh VM cache using the Proxmox client
	proxmoxClient := h.stateManager.GetProxmoxClient()
	if proxmoxClient != nil {
		proxmoxClient.InvalidateCache("/nodes/" + node + "/qemu")
		proxmoxClient.InvalidateCache("/nodes/" + node + "/qemu/" + vmidStr)
		log.Info().Str("node", node).Str("vmid", vmidStr).Msg("Invalidated VM caches after snapshot operation")
	}

	http.Redirect(w, r, "/vm/details?vmid="+vmidStr+"&node="+node+"&success=snapshot_created", http.StatusFound)
}

// UpdateVMSnapshotHandler handles updating a snapshot description
func (h *VMSnapshotsHandler) UpdateVMSnapshotHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	log := logger.Get()

	if !ValidateMethodAndParseForm(w, r, http.MethodPost) {
		return
	}

	// Get VM details from form
	vmidStr := r.FormValue("vmid")
	node := r.FormValue("node")
	snapshotName := r.FormValue("snapshot_name")
	description := r.FormValue("description")

	if vmidStr == "" || node == "" || snapshotName == "" {
		log.Error().Msg("Missing required parameters for snapshot update")
		http.Redirect(w, r, "/vm/details?vmid="+vmidStr+"&node="+node+"&error=missing_parameters", http.StatusFound)
		return
	}

	// Get Proxmox client
	restyClient, err := getDefaultRestyClient()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get Proxmox client")
		http.Redirect(w, r, "/vm/details?vmid="+vmidStr+"&node="+node+"&error=client_error", http.StatusFound)
		return
	}

	// Update snapshot description
	err = proxmox.UpdateVMSnapshotResty(r.Context(), restyClient, node, vmidStr, snapshotName, description)
	if err != nil {
		log.Error().Err(err).Str("snapshot_name", snapshotName).Msg("Failed to update snapshot")
		http.Redirect(w, r, "/vm/details?vmid="+vmidStr+"&node="+node+"&error=update_failed", http.StatusFound)
		return
	}

	log.Info().
		Str("vmid", vmidStr).
		Str("node", node).
		Str("snapshot_name", snapshotName).
		Msg("VM snapshot updated successfully")

	http.Redirect(w, r, "/vm/details?vmid="+vmidStr+"&node="+node+"&success=snapshot_updated", http.StatusFound)
}

// DeleteVMSnapshotHandler handles deleting a VM snapshot
func (h *VMSnapshotsHandler) DeleteVMSnapshotHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	log := logger.Get()

	if !ValidateMethodAndParseForm(w, r, http.MethodPost) {
		return
	}

	// Get VM details from form
	vmidStr := r.FormValue("vmid")
	node := r.FormValue("node")
	snapshotName := r.FormValue("snapshot_name")

	if vmidStr == "" || node == "" || snapshotName == "" {
		log.Error().Msg("Missing required parameters for snapshot deletion")
		http.Redirect(w, r, "/vm/details?vmid="+vmidStr+"&node="+node+"&error=missing_parameters", http.StatusFound)
		return
	}

	// Get Proxmox client
	restyClient, err := getDefaultRestyClient()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get Proxmox client")
		http.Redirect(w, r, "/vm/details?vmid="+vmidStr+"&node="+node+"&error=client_error", http.StatusFound)
		return
	}

	// Delete snapshot
	err = proxmox.DeleteVMSnapshotResty(r.Context(), restyClient, node, vmidStr, snapshotName)
	if err != nil {
		log.Error().Err(err).Str("snapshot_name", snapshotName).Msg("Failed to delete snapshot")
		http.Redirect(w, r, "/vm/details?vmid="+node+"&vmid="+vmidStr+"&error=delete_failed", http.StatusFound)
		return
	}

	log.Info().
		Str("vmid", vmidStr).
		Str("node", node).
		Str("snapshot_name", snapshotName).
		Msg("VM snapshot deleted successfully")

	// Force refresh VM cache using the Proxmox client
	proxmoxClient := h.stateManager.GetProxmoxClient()
	if proxmoxClient != nil {
		proxmoxClient.InvalidateCache("/nodes/" + node + "/qemu")
		proxmoxClient.InvalidateCache("/nodes/" + node + "/qemu/" + vmidStr)
		log.Info().Str("node", node).Str("vmid", vmidStr).Msg("Invalidated VM caches after snapshot operation")
	}

	http.Redirect(w, r, "/vm/details?vmid="+vmidStr+"&node="+node+"&success=snapshot_deleted", http.StatusFound)
}

// RollbackVMSnapshotHandler handles rolling back a VM to a snapshot
func (h *VMSnapshotsHandler) RollbackVMSnapshotHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	log := logger.Get()

	if !ValidateMethodAndParseForm(w, r, http.MethodPost) {
		return
	}

	// Get VM details from form
	vmidStr := r.FormValue("vmid")
	node := r.FormValue("node")
	snapshotName := r.FormValue("snapshot_name")

	if vmidStr == "" || node == "" || snapshotName == "" {
		log.Error().Msg("Missing required parameters for snapshot rollback")
		http.Redirect(w, r, "/vm/details?vmid="+vmidStr+"&node="+node+"&error=missing_parameters", http.StatusFound)
		return
	}

	// Get Proxmox client
	restyClient, err := getDefaultRestyClient()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get Proxmox client")
		http.Redirect(w, r, "/vm/details?vmid="+vmidStr+"&node="+node+"&error=client_error", http.StatusFound)
		return
	}

	// Rollback to snapshot
	err = proxmox.RollbackVMSnapshotResty(r.Context(), restyClient, node, vmidStr, snapshotName)
	if err != nil {
		log.Error().Err(err).Str("snapshot_name", snapshotName).Msg("Failed to rollback snapshot")
		http.Redirect(w, r, "/vm/details?vmid="+vmidStr+"&node="+node+"&error=rollback_failed", http.StatusFound)
		return
	}

	log.Info().
		Str("vmid", vmidStr).
		Str("node", node).
		Str("snapshot_name", snapshotName).
		Msg("VM rolled back to snapshot successfully")

	// Force refresh VM cache using the Proxmox client
	proxmoxClient := h.stateManager.GetProxmoxClient()
	if proxmoxClient != nil {
		proxmoxClient.InvalidateCache("/nodes/" + node + "/qemu")
		proxmoxClient.InvalidateCache("/nodes/" + node + "/qemu/" + vmidStr)
		log.Info().Str("node", node).Str("vmid", vmidStr).Msg("Invalidated VM caches after snapshot operation")
	}

	http.Redirect(w, r, "/vm/details?vmid="+vmidStr+"&node="+node+"&success=snapshot_rollback", http.StatusFound)
}

// GetVMSnapshotsHandler returns VM snapshots as JSON for AJAX requests
func (h *VMSnapshotsHandler) GetVMSnapshotsHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	log := logger.Get()

	if !ValidateMethodAndParseForm(w, r, http.MethodGet) {
		return
	}

	vmidStr := r.URL.Query().Get("vmid")
	node := r.URL.Query().Get("node")

	if vmidStr == "" || node == "" {
		log.Error().Msg("Missing required parameters for getting snapshots")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		if _, err := w.Write([]byte(`{"error": "Missing required parameters"}`)); err != nil {
			log.Error().Err(err).Msg("Failed to write error response")
		}
		return
	}

	// Get Proxmox client
	restyClient, err := getDefaultRestyClient()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get Proxmox client")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		if _, err := w.Write([]byte(`{"error": "Failed to get Proxmox client"}`)); err != nil {
			log.Error().Err(err).Msg("Failed to write error response")
		}
		return
	}

	// Get snapshots
	snapshots, err := proxmox.GetVMSnapshotsResty(r.Context(), restyClient, node, vmidStr)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get snapshots")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		if _, err := w.Write([]byte(`{"error": "Failed to get snapshots"}`)); err != nil {
			log.Error().Err(err).Msg("Failed to write error response")
		}
		return
	}

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	response := `{"snapshots": [`
	for i, snapshot := range snapshots {
		if i > 0 {
			response += ","
		}
		response += `{"name":"` + snapshot.Name + `","description":"` + snapshot.Description + `","snaptime":` + strconv.FormatInt(snapshot.Snaptime, 10) + `,"vmstate":` + strconv.Itoa(snapshot.Vmstate) + `}`
	}
	response += `]}`
	if _, err := w.Write([]byte(response)); err != nil {
		log.Error().Err(err).Msg("Failed to write JSON response")
	}
}

// ValidateSnapshotName validates snapshot name format (a-zA-Z0-9-_)
func ValidateSnapshotName(name string) bool {
	if len(name) == 0 || len(name) > 40 {
		return false
	}

	// Only allow alphanumeric characters, hyphens, and underscores
	validName := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	return validName.MatchString(name)
}

// SanitizeSnapshotName removes invalid characters and replaces spaces with underscores
func SanitizeSnapshotName(name string) string {
	// Replace spaces with underscores
	name = strings.ReplaceAll(name, " ", "_")

	// Remove invalid characters, keeping only a-zA-Z0-9-_
	validName := regexp.MustCompile(`[^a-zA-Z0-9_-]`)
	return validName.ReplaceAllString(name, "")
}
