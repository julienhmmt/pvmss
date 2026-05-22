package apiv1

import "net/http"

// ToggleStorage handles POST /api/v1/admin/storage/toggle.
func (h *AdminMutationsHandler) ToggleStorage(w http.ResponseWriter, r *http.Request) {
	var req ToggleStorageRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Storage == "" || req.Node == "" {
		errBadRequest(w, "storage and node are required")
		return
	}

	uniqueID := req.Node + ":" + req.Storage

	settings := h.state.GetSettings()
	newStorages := make([]string, len(settings.EnabledStorages))
	copy(newStorages, settings.EnabledStorages)

	found := false
	for _, s := range newStorages {
		if s == uniqueID {
			found = true
			break
		}
	}

	if found {
		filtered := make([]string, 0, len(newStorages))
		for _, s := range newStorages {
			if s != uniqueID {
				filtered = append(filtered, s)
			}
		}
		newStorages = filtered
	} else {
		newStorages = append(newStorages, uniqueID)
	}

	if h.state.HasDB() {
		if err := h.state.SetEnabledStorages(newStorages, usernameFromCtx(r)); err != nil {
			writeAppError(w, err)
			return
		}
	} else {
		newSettings := *settings
		newSettings.EnabledStorages = newStorages
		newSettings.Limits.Nodes = copyNodeLimits(settings.Limits.Nodes)
		if err := h.state.SetSettings(&newSettings); err != nil {
			writeAppError(w, err)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// ToggleVMBR handles POST /api/v1/admin/vmbr/toggle.
func (h *AdminMutationsHandler) ToggleVMBR(w http.ResponseWriter, r *http.Request) {
	var req ToggleVMBRRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if req.VMBR == "" || req.Node == "" {
		errBadRequest(w, "vmbr and node are required")
		return
	}

	uniqueID := req.Node + ":" + req.VMBR

	settings := h.state.GetSettings()
	newVMBRs := make([]string, len(settings.VMBRs))
	copy(newVMBRs, settings.VMBRs)

	found := false
	for _, v := range newVMBRs {
		if v == uniqueID {
			found = true
			break
		}
	}

	if found {
		filtered := make([]string, 0, len(newVMBRs))
		for _, v := range newVMBRs {
			if v != uniqueID {
				filtered = append(filtered, v)
			}
		}
		newVMBRs = filtered
	} else {
		newVMBRs = append(newVMBRs, uniqueID)
	}

	if h.state.HasDB() {
		if err := h.state.SetEnabledVMBRs(newVMBRs, usernameFromCtx(r)); err != nil {
			writeAppError(w, err)
			return
		}
	} else {
		newSettings := *settings
		newSettings.VMBRs = newVMBRs
		newSettings.Limits.Nodes = copyNodeLimits(settings.Limits.Nodes)
		if err := h.state.SetSettings(&newSettings); err != nil {
			writeAppError(w, err)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// ToggleISO handles POST /api/v1/admin/iso/toggle.
func (h *AdminMutationsHandler) ToggleISO(w http.ResponseWriter, r *http.Request) {
	var req ToggleISORequest
	if !decodeBody(w, r, &req) {
		return
	}
	if req.VolID == "" {
		errBadRequest(w, "volid is required")
		return
	}

	settings := h.state.GetSettings()
	newISOs := make([]string, len(settings.ISOs))
	copy(newISOs, settings.ISOs)

	found := false
	for _, iso := range newISOs {
		if iso == req.VolID {
			found = true
			break
		}
	}

	if found {
		filtered := make([]string, 0, len(newISOs))
		for _, iso := range newISOs {
			if iso != req.VolID {
				filtered = append(filtered, iso)
			}
		}
		newISOs = filtered
	} else {
		newISOs = append(newISOs, req.VolID)
	}

	if h.state.HasDB() {
		if err := h.state.SetEnabledISOs(newISOs, usernameFromCtx(r)); err != nil {
			writeAppError(w, err)
			return
		}
	} else {
		newSettings := *settings
		newSettings.ISOs = newISOs
		newSettings.Limits.Nodes = copyNodeLimits(settings.Limits.Nodes)
		if err := h.state.SetSettings(&newSettings); err != nil {
			writeAppError(w, err)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}
