package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

// sendVNCJSONResponse sends a JSON response for VNC API calls
func sendVNCJSONResponse(w http.ResponseWriter, statusCode int, success bool, data map[string]interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	response := map[string]interface{}{"success": success}
	for k, v := range data {
		response[k] = v
	}
	_ = json.NewEncoder(w).Encode(response)
}

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
		sendVNCJSONResponse(w, http.StatusUnauthorized, false, map[string]interface{}{
			"error": "Proxmox authentication expired. Please log in again.",
		})
		return
	}

	// Get parameters
	vmid := r.URL.Query().Get("vmid")
	node := r.URL.Query().Get("node")

	if vmid == "" || node == "" {
		log.Warn().Str("vmid", vmid).Str("node", node).Msg("Missing required parameters")
		sendVNCJSONResponse(w, http.StatusBadRequest, false, map[string]interface{}{
			"error": "Missing vmid or node parameter",
		})
		return
	}

	log.Info().Str("vmid", vmid).Str("node", node).Msg("Requesting VNC proxy ticket")

	// Get VNC proxy ticket using stored user credentials
	ticket, port, err := GetVNCProxyTicket(r, node, vmid)
	if err != nil {
		log.Error().Err(err).Str("vmid", vmid).Str("node", node).Msg("Failed to get VNC proxy ticket")
		LogVNCConsoleAccess(r, vmid, node, false)
		sendVNCJSONResponse(w, http.StatusInternalServerError, false, map[string]interface{}{
			"error": "Failed to create console session. Please ensure you have permission to access this VM.",
		})
		return
	}

	LogVNCConsoleAccess(r, vmid, node, true)

	log.Info().
		Str("vmid", vmid).
		Str("node", node).
		Int("port", port).
		Msg("VNC proxy ticket created successfully")

	// Return JSON response
	sendVNCJSONResponse(w, http.StatusOK, true, map[string]interface{}{
		"ticket": ticket,
		"port":   port,
		"node":   node,
		"vmid":   vmid,
	})
}
