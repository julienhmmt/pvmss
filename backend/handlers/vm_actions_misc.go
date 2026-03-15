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
func (h *VMHandler) UpdateVMDescriptionHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx := HandlerContextWith(w, r, "UpdateVMDescriptionHandler")

	if !ValidateMethodAndParseForm(w, r, http.MethodPost) {
		return
	}
	vmid := strings.TrimSpace(r.FormValue("vmid"))
	node := strings.TrimSpace(r.FormValue("node"))
	desc := r.FormValue("description")
	{
		s := MakeInputSanitizer()
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

	restyClient, err := getDefaultRestyClient()
	if err != nil {
		ctx.HandleError(err, "Failed to create Proxmox client", http.StatusInternalServerError)
		return
	}

	if err := proxmox.UpdateVMConfigResty(r.Context(), restyClient, node, vmidInt, map[string]string{"description": desc}); err != nil {
		ctx.Log.Error().Err(err).Msg("update description failed")
		ctx.RedirectWithError(buildVMDetailsURL(vmid), "Message.ActionFailed")
		return
	}
	ctx.Log.Info().Str("vmid", vmid).Str("node", node).Msg("VM description updated successfully")
	ctx.RedirectWithSuccess(buildVMDetailsURL(vmid), "Message.UpdatedSuccessfully")
}

// UpdateVMTagsHandler updates the VM tags from selected checkboxes.
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
		s := MakeInputSanitizer()
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

	restyClient, err := getDefaultRestyClient()
	if err != nil {
		ctx.HandleError(err, "Failed to create Proxmox client", http.StatusInternalServerError)
		return
	}

	if err := proxmox.UpdateVMConfigResty(r.Context(), restyClient, node, vmidInt, map[string]string{"tags": tagsStr}); err != nil {
		ctx.Log.Error().Err(err).Msg("update tags failed")
		ctx.RedirectWithError(buildVMDetailsURL(vmid), "Message.ActionFailed")
		return
	}
	ctx.RedirectWithSuccess(buildVMDetailsURL(vmid), "Message.UpdatedSuccessfully")
}
