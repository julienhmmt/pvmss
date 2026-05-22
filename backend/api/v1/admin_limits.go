package apiv1

import (
	"net/http"

	"pvmss/database"
	"pvmss/state"
)

// GetLimits handles GET /api/v1/admin/limits.
func (h *AdminMutationsHandler) GetLimits(w http.ResponseWriter, _ *http.Request) {
	settings := h.state.GetSettings()
	nodes := make(map[string]NodeResourceLimitsResponse, len(settings.Limits.Nodes))
	for k, v := range settings.Limits.Nodes {
		nodes[k] = NodeResourceLimitsResponse{
			Sockets: ResourceRangeResponse{Min: v.Sockets.Min, Max: v.Sockets.Max},
			Cores:   ResourceRangeResponse{Min: v.Cores.Min, Max: v.Cores.Max},
			RAM:     ResourceRangeResponse{Min: v.RAM.Min, Max: v.RAM.Max},
			Disk:    ResourceRangeResponse{Min: v.Disk.Min, Max: v.Disk.Max},
			MaxVMs:  v.MaxVMs,
		}
	}
	writeJSON(w, AdminLimitsResponse{
		VM: VMResourceLimitsResponse{
			Sockets: ResourceRangeResponse{Min: settings.Limits.VM.Sockets.Min, Max: settings.Limits.VM.Sockets.Max},
			Cores:   ResourceRangeResponse{Min: settings.Limits.VM.Cores.Min, Max: settings.Limits.VM.Cores.Max},
			RAM:     ResourceRangeResponse{Min: settings.Limits.VM.RAM.Min, Max: settings.Limits.VM.RAM.Max},
			Disk:    ResourceRangeResponse{Min: settings.Limits.VM.Disk.Min, Max: settings.Limits.VM.Disk.Max},
		},
		Nodes:           nodes,
		MaxSnapshots:    settings.Limits.MaxSnapshots,
		MaxNetworkCards: settings.MaxNetworkCards,
		MaxDiskPerVM:    settings.MaxDiskPerVM,
		MaxVMPerUser:    settings.MaxVMPerUser,
	})
}

// UpdateLimits handles PUT /api/v1/admin/limits.
func (h *AdminMutationsHandler) UpdateLimits(w http.ResponseWriter, r *http.Request) {
	var req AdminLimitsResponse
	if !decodeBody(w, r, &req) {
		return
	}

	changedBy := usernameFromCtx(r)

	if h.state.HasDB() {
		current := h.state.GetSettings()
		limits := &database.VMLimits{
			MaxVMs:          0,
			MaxVMPerUser:    req.MaxVMPerUser,
			MaxNetworkCards: req.MaxNetworkCards,
			MaxDiskPerVM:    req.MaxDiskPerVM,
			AllowCustomYAML: current.AllowCustomYAML,
			MaxSnapshots:    req.MaxSnapshots,
		}
		if err := h.state.SetVMLimits(limits, changedBy); err != nil {
			writeAppError(w, err)
			return
		}
		for nodeName, nodeReq := range req.Nodes {
			if nodeReq.MaxVMs > 0 {
				existing, found, err := h.state.GetNodeLimitFromDB(nodeName)
				if err != nil {
					writeAppError(w, err)
					return
				}
				if !found {
					existing = database.NodeLimit{NodeName: nodeName}
				}
				if err := h.state.SetNodeLimit(database.NodeLimit{
					NodeName:  nodeName,
					MaxVMs:    nodeReq.MaxVMs,
					MaxVCPUs:  existing.MaxVCPUs,
					MaxRAMGB:  existing.MaxRAMGB,
					MaxDiskGB: existing.MaxDiskGB,
				}, changedBy); err != nil {
					writeAppError(w, err)
					return
				}
			} else {
				existing, found, err := h.state.GetNodeLimitFromDB(nodeName)
				if err != nil {
					writeAppError(w, err)
					return
				}
				if found && (existing.MaxVCPUs > 0 || existing.MaxRAMGB > 0 || existing.MaxDiskGB > 0) {
					if err := h.state.SetNodeLimit(database.NodeLimit{
						NodeName:  nodeName,
						MaxVMs:    0,
						MaxVCPUs:  existing.MaxVCPUs,
						MaxRAMGB:  existing.MaxRAMGB,
						MaxDiskGB: existing.MaxDiskGB,
					}, changedBy); err != nil {
						writeAppError(w, err)
						return
					}
				} else {
					_ = h.state.DeleteNodeLimit(nodeName, changedBy)
				}
			}
		}
	} else {
		settings := h.state.GetSettings()
		newSettings := *settings
		newSettings.Limits = state.LimitsConfig{
			VM: state.VMResourceLimits{
				Sockets: state.ResourceRange{Min: req.VM.Sockets.Min, Max: req.VM.Sockets.Max},
				Cores:   state.ResourceRange{Min: req.VM.Cores.Min, Max: req.VM.Cores.Max},
				RAM:     state.ResourceRange{Min: req.VM.RAM.Min, Max: req.VM.RAM.Max},
				Disk:    state.ResourceRange{Min: req.VM.Disk.Min, Max: req.VM.Disk.Max},
			},
			Nodes:        make(map[string]state.NodeResourceLimits, len(req.Nodes)),
			MaxSnapshots: req.MaxSnapshots,
		}
		for k, v := range req.Nodes {
			newSettings.Limits.Nodes[k] = state.NodeResourceLimits{
				Sockets: state.ResourceRange{Min: v.Sockets.Min, Max: v.Sockets.Max},
				Cores:   state.ResourceRange{Min: v.Cores.Min, Max: v.Cores.Max},
				RAM:     state.ResourceRange{Min: v.RAM.Min, Max: v.RAM.Max},
				Disk:    state.ResourceRange{Min: v.Disk.Min, Max: v.Disk.Max},
				MaxVMs:  v.MaxVMs,
			}
		}
		newSettings.MaxNetworkCards = req.MaxNetworkCards
		newSettings.MaxDiskPerVM = req.MaxDiskPerVM
		newSettings.MaxVMPerUser = req.MaxVMPerUser
		if err := h.state.SetSettings(&newSettings); err != nil {
			writeAppError(w, err)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}
