package handlers

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/julienschmidt/httprouter"

	"pvmss/proxmox"
)

// UpdateVMDescriptionHandler updates the VM description (Markdown supported on display).
// TODO Telmate migration: this handler still uses Telmate-based VM config helpers. Replace with Resty UpdateVMConfigResty.
func (h *VMHandler) UpdateVMDescriptionHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx := HandlerContextWith(w, r, "UpdateVMDescriptionHandler")

	if !ValidateMethodAndParseForm(w, r, http.MethodPost) {
		return
	}
	vmid := strings.TrimSpace(r.FormValue("vmid"))
	node := strings.TrimSpace(r.FormValue("node"))
	desc := r.FormValue("description")
	{
		s := NewInputSanitizer()
		desc = s.RemoveScriptTags(s.SanitizeString(desc, 2000))
	}
	if !IsAuthenticated(r) {
		returnTo := "/"
		if vmid != "" {
			returnTo = "/vm/details/" + vmid + "?edit=description"
		}
		http.Redirect(w, r, "/login?warning=login_required&context=update_description&return="+url.QueryEscape(returnTo), http.StatusSeeOther)
		return
	}
	if vmid == "" || node == "" {
		ctx.HandleError(nil, "Bad request", http.StatusBadRequest)
		return
	}
	vmidInt, err := strconv.Atoi(vmid)
	if err != nil {
		ctx.HandleError(err, "Invalid VM ID", http.StatusBadRequest)
		return
	}

	client := ctx.StateManager.GetProxmoxClient()
	if client == nil {
		ctx.HandleError(nil, "Proxmox client not available", http.StatusInternalServerError)
		return
	}

	if err := proxmox.UpdateVMConfigWithContext(r.Context(), client, node, vmidInt, map[string]string{"description": desc}); err != nil {
		ctx.Log.Error().Err(err).Msg("update description failed")
		ctx.RedirectWithError(buildVMDetailsURL(vmid), "Message.ActionFailed")
		return
	}
	ctx.Log.Info().Str("vmid", vmid).Str("node", node).Msg("VM description updated successfully")
	ctx.RedirectWithSuccess(buildVMDetailsURL(vmid), "Message.UpdatedSuccessfully")
}

// UpdateVMTagsHandler updates the VM tags from selected checkboxes.
// TODO Telmate migration: replace Telmate config update with Resty UpdateVMConfigResty.
func (h *VMHandler) UpdateVMTagsHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx := HandlerContextWith(w, r, "UpdateVMTagsHandler")

	if !ValidateMethodAndParseForm(w, r, http.MethodPost) {
		return
	}
	vmid := strings.TrimSpace(r.FormValue("vmid"))
	node := strings.TrimSpace(r.FormValue("node"))
	if vmid == "" || node == "" {
		ctx.HandleError(nil, "Bad request", http.StatusBadRequest)
		return
	}
	vmidInt, err := strconv.Atoi(vmid)
	if err != nil {
		ctx.HandleError(err, "Invalid VM ID", http.StatusBadRequest)
		return
	}

	selectedTags := r.Form["tags"]
	if len(selectedTags) > 0 {
		s := NewInputSanitizer()
		cleaned := make([]string, 0, len(selectedTags))
		for _, t := range selectedTags {
			st := s.SanitizeString(strings.TrimSpace(t), 64)
			if st != "" {
				cleaned = append(cleaned, st)
			}
		}
		selectedTags = cleaned
	}
	tagsStr := strings.Join(selectedTags, ";")

	client := ctx.StateManager.GetProxmoxClient()
	if client == nil {
		ctx.HandleError(nil, "Proxmox client not available", http.StatusInternalServerError)
		return
	}

	if err := proxmox.UpdateVMConfigWithContext(r.Context(), client, node, vmidInt, map[string]string{"tags": tagsStr}); err != nil {
		ctx.Log.Error().Err(err).Msg("update tags failed")
		ctx.RedirectWithError(buildVMDetailsURL(vmid), "Message.ActionFailed")
		return
	}
	ctx.RedirectWithSuccess(buildVMDetailsURL(vmid), "Message.UpdatedSuccessfully")
}
