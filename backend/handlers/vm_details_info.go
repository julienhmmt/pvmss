package handlers

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
)

// VMDetailsHandler renders the Vue SPA shell for the VM details page.
// All VM data is fetched by the Vue frontend via /api/v1/vms/:vmid.
func (h *VMHandler) VMDetailsHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	vmid := ps.ByName("vmid")
	if vmid == "" {
		http.Error(w, "VM ID required", http.StatusBadRequest)
		return
	}
	ctx := NewHandlerContext(w, r, "VMDetailsHandler")
	renderVueShell(w, r, ctx, "VM Details")
}
