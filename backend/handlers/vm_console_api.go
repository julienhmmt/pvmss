package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

// GetVNCTicketHandler returns a VNC proxy ticket for the specified VM.
// This endpoint creates a VNC proxy session with Proxmox and returns the ticket and port.
//
// POST /api/vm/vnc-ticket?vmid={vmid}&node={node}
//
// Response:
//
//	{
//	  "success": true,
//	  "ticket": "PVEVNC:...",
//	  "port": 5900,
//	  "node": "pve1",
//	  "vmid": "100"
//	}
func (h *VMHandler) GetVNCTicketHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("GetVNCTicketHandler", r)

	if !ValidateMethodAndParseForm(w, r, http.MethodPost) {
		return
	}

	// Validate authentication
	if !IsAuthenticated(r) {
		log.Warn().Msg("Unauthenticated VNC ticket request")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Check Proxmox ticket validity
	if !IsProxmoxTicketValid(r) {
		log.Warn().Msg("Proxmox ticket expired or invalid")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Proxmox authentication expired. Please log in again.",
		})
		return
	}

	// Get parameters
	vmid := r.URL.Query().Get("vmid")
	node := r.URL.Query().Get("node")

	if vmid == "" || node == "" {
		log.Warn().Str("vmid", vmid).Str("node", node).Msg("Missing required parameters")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Missing vmid or node parameter",
		})
		return
	}

	log.Info().Str("vmid", vmid).Str("node", node).Msg("Requesting VNC proxy ticket")

	// Get VNC proxy ticket using stored user credentials
	ticket, port, err := GetVNCProxyTicket(r, node, vmid)
	if err != nil {
		log.Error().Err(err).Str("vmid", vmid).Str("node", node).Msg("Failed to get VNC proxy ticket")

		// Log failed access attempt
		LogVNCConsoleAccess(r, vmid, node, false, &log)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Failed to create console session. Please ensure you have permission to access this VM.",
		})
		return
	}

	// Log successful access
	LogVNCConsoleAccess(r, vmid, node, true, &log)

	log.Info().
		Str("vmid", vmid).
		Str("node", node).
		Int("port", port).
		Msg("VNC proxy ticket created successfully")

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"ticket":  ticket,
		"port":    port,
		"node":    node,
		"vmid":    vmid,
	})
}
