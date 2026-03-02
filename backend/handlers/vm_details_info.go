package handlers

import (
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"

	"pvmss/i18n"
)

// VMDetailsHandler renders the VM details page (Vue SPA shell).
// All VM data is loaded client-side via /api/v1/vms/:id and /api/v1/vms/:id/metrics.
func (h *VMHandler) VMDetailsHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	vmid := ps.ByName("vmid")
	if vmid == "" {
		http.Error(w, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.MissingRequiredFields"), http.StatusBadRequest)
		return
	}
	if _, err := strconv.Atoi(vmid); err != nil {
		http.Error(w, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.Generic"), http.StatusBadRequest)
		return
	}
	renderVueShell(w, r, "VM Details", "/vm/details/"+vmid)
}
