package handlers

import (
	"pvmss/proxmox"
	"pvmss/state"

	"github.com/julienschmidt/httprouter"
)

// VMStateManager defines the minimal state contract needed by VM details.
// Provides access to Proxmox client and application settings.
type VMStateManager interface {
	GetProxmoxClient() proxmox.ClientInterface
	GetSettings() *state.AppSettings
	GetProxmoxStatus() (bool, string)
}

// VMHandler handles VM-related pages and API endpoints.
type VMHandler struct {
	stateManager state.StateManager
}

// NewVMHandler creates a new VMHandler.
func NewVMHandler(stateManager state.StateManager) *VMHandler {
	return &VMHandler{stateManager: stateManager}
}

// RegisterRoutes registers VM-related routes.
func (h *VMHandler) RegisterRoutes(router *httprouter.Router) {
	router.GET("/vm/details/:vmid", RequireAuthHandle(h.VMDetailsHandler))

	// API routes for dynamic updates
	router.GET("/api/vm/:vmid/metrics", RequireAuthHandle(h.VMMetricsHandler))

	router.POST("/api/vm/validate/vmid", RequireAuthHandle(h.ValidateVMIDHandler))
	router.POST("/api/vm/validate/name", RequireAuthHandle(h.ValidateVMNameHandler))
	router.POST("/api/vm/validate/vlan_tag", RequireAuthHandle(h.ValidateVLANHandler))

	router.POST("/vm/update/description", SecureFormHandler("UpdateVMDescription",
		RequireAuthHandle(h.UpdateVMDescriptionHandler),
	))
	router.POST("/vm/update/tags", SecureFormHandler("UpdateVMTags",
		RequireAuthHandle(h.UpdateVMTagsHandler),
	))
	router.POST("/vm/update/resources", SecureFormHandler("UpdateVMResources",
		RequireAuthHandle(h.UpdateVMResourcesHandler),
	))
	router.POST("/vm/toggle/network", SecureFormHandler("ToggleNetworkCard",
		RequireAuthHandle(h.ToggleNetworkCardHandler),
	))

	// Snapshot routes
	snapshotHandler := NewVMSnapshotsHandler(h.stateManager)
	router.POST("/vm/snapshot/create", SecureFormHandler("CreateVMSnapshot",
		RequireAuthHandle(snapshotHandler.CreateVMSnapshotHandler),
	))
	router.POST("/vm/snapshot/update", SecureFormHandler("UpdateVMSnapshot",
		RequireAuthHandle(snapshotHandler.UpdateVMSnapshotHandler),
	))
	router.POST("/vm/snapshot/delete", SecureFormHandler("DeleteVMSnapshot",
		RequireAuthHandle(snapshotHandler.DeleteVMSnapshotHandler),
	))
	router.POST("/vm/snapshot/rollback", SecureFormHandler("RollbackVMSnapshot",
		RequireAuthHandle(snapshotHandler.RollbackVMSnapshotHandler),
	))
	router.GET("/api/vm/snapshots", RequireAuthHandle(snapshotHandler.GetVMSnapshotsHandler))

	router.POST("/vm/action", SecureFormHandler("VMAction",
		RequireAuthHandle(h.VMActionHandler),
	))

	// VM deletion routes
	router.GET("/vm/delete/:vmid", RequireAuthHandle(h.VMDeleteConfirmHandler))
	router.POST("/vm/delete", RequireAuthHandle(h.VMDeleteHandler))

	// VM console routes
	router.POST("/api/vm/vnc-ticket", RequireAuthHandle(h.GetVNCTicketHandler))
	router.GET("/vm/console/websocket", RequireAuthHandle(h.VMConsoleWebSocketHandler))
}
