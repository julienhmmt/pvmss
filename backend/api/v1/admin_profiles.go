package apiv1

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/julienschmidt/httprouter"

	"pvmss/database"
	"pvmss/state"
)

var profileIDRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,49}$`)
var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

var validDiskBuses = map[string]bool{"virtio": true, "scsi": true, "sata": true, "ide": true}
var validProfileIcons = map[string]bool{"Globe": true, "Code": true, "Cube": true, "Database": true, "Flask": true, "Monitor": true, "Cpu": true, "HardDrive": true, "Cloud": true, "Info": true}
var validProfileColors = map[string]bool{"blue": true, "violet": true, "emerald": true, "teal": true, "amber": true, "rose": true, "indigo": true, "sky": true, "orange": true, "gray": true}

func validateVMProfile(p *state.VMProfileConfig) error {
	if !profileIDRegex.MatchString(p.ID) {
		return fmt.Errorf("id must be lowercase alphanumeric with hyphens (max 50 chars, start with alphanumeric)")
	}
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if p.Sockets < 1 || p.Sockets > 8 {
		return fmt.Errorf("sockets must be between 1 and 8")
	}
	if p.Cores < 1 || p.Cores > 64 {
		return fmt.Errorf("cores must be between 1 and 64")
	}
	if p.RAMGB < 1 || p.RAMGB > 512 {
		return fmt.Errorf("ram_gb must be between 1 and 512")
	}
	if p.DiskGB < 1 || p.DiskGB > 2000 {
		return fmt.Errorf("disk_gb must be between 1 and 2000")
	}
	if !validDiskBuses[p.DiskBus] {
		return fmt.Errorf("disk_bus must be one of: virtio, scsi, sata, ide")
	}
	if !validProfileIcons[p.Icon] {
		return fmt.Errorf("icon must be one of: Globe, Code, Cube, Database, Flask, Monitor, Cpu, HardDrive, Cloud, Info")
	}
	if !validProfileColors[p.Color] {
		return fmt.Errorf("color must be one of: blue, violet, emerald, teal, amber, rose, indigo, sky, orange, gray")
	}
	return nil
}

// slugifyProfile converts a name to a safe profile ID.
func slugifyProfile(name string) string {
	slug := strings.ToLower(strings.TrimSpace(name))
	slug = slugRe.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if len(slug) > 50 {
		slug = slug[:50]
	}
	if slug == "" {
		return "profile"
	}
	return slug
}

// VMProfileListResponse is the response for GET /api/v1/admin/vm-profiles.
type VMProfileListResponse struct {
	Profiles      []state.VMProfileConfig `json:"profiles"`
	UsingDefaults bool                    `json:"using_defaults"`
}

// ListVMProfiles handles GET /api/v1/admin/vm-profiles.
func (h *AdminMutationsHandler) ListVMProfiles(w http.ResponseWriter, _ *http.Request) {
	settings := h.state.GetSettings()
	usingDefaults := len(settings.VMProfiles) == 0
	writeJSON(w, VMProfileListResponse{
		Profiles:      settings.GetVMProfiles(),
		UsingDefaults: usingDefaults,
	})
}

// vmProfileConfigToDB converts a state.VMProfileConfig to a database.VMProfile
// by marshalling the config fields into a JSON blob.
func vmProfileConfigToDB(p state.VMProfileConfig) (*database.VMProfile, error) {
	blob := database.VMProfileConfigBlob{
		Sockets: p.Sockets, Cores: p.Cores, RAMGB: p.RAMGB,
		DiskGB: p.DiskGB, DiskBus: p.DiskBus, Node: p.Node,
		Storage: p.Storage, Icon: p.Icon, Color: p.Color,
		EnableEFI: p.EnableEFI,
	}
	configBytes, err := json.Marshal(blob)
	if err != nil {
		return nil, fmt.Errorf("marshal profile config: %w", err)
	}
	return &database.VMProfile{
		ID: p.ID, Name: p.Name, Description: p.Description,
		Config: string(configBytes), Enabled: p.Enabled,
	}, nil
}

// CreateVMProfile handles POST /api/v1/admin/vm-profiles.
func (h *AdminMutationsHandler) CreateVMProfile(w http.ResponseWriter, r *http.Request) {
	var req state.VMProfileConfig
	if !decodeBody(w, r, &req) {
		return
	}
	req.ID = strings.TrimSpace(req.ID)
	req.Name = strings.TrimSpace(req.Name)
	if req.ID == "" {
		req.ID = slugifyProfile(req.Name)
	}
	if err := validateVMProfile(&req); err != nil {
		errBadRequest(w, err.Error())
		return
	}
	settings := h.state.GetSettings()
	profiles := settings.GetVMProfiles()
	for _, p := range profiles {
		if p.ID == req.ID {
			errBadRequest(w, "a profile with this ID already exists")
			return
		}
	}
	if h.state.HasDB() {
		dbProfile, err := vmProfileConfigToDB(req)
		if err != nil {
			writeAppError(w, err)
			return
		}
		if err := h.state.CreateVMProfile(dbProfile, usernameFromCtx(r)); err != nil {
			writeAppError(w, err)
			return
		}
	} else {
		newSettings := *settings
		if len(newSettings.VMProfiles) == 0 {
			newSettings.VMProfiles = state.DefaultVMProfiles()
		}
		newSettings.VMProfiles = copyVMProfiles(newSettings.VMProfiles)
		newSettings.Limits.Nodes = copyNodeLimits(settings.Limits.Nodes)
		newSettings.VMProfiles = append(newSettings.VMProfiles, req)
		if err := h.state.SetSettings(&newSettings); err != nil {
			writeAppError(w, err)
			return
		}
	}
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, req)
}

// UpdateVMProfile handles PUT /api/v1/admin/vm-profiles/:id.
func (h *AdminMutationsHandler) UpdateVMProfile(w http.ResponseWriter, r *http.Request) {
	ps := httprouter.ParamsFromContext(r.Context())
	id := ps.ByName("id")
	if id == "" {
		errBadRequest(w, "missing profile id")
		return
	}
	var req state.VMProfileConfig
	if !decodeBody(w, r, &req) {
		return
	}
	req.ID = strings.TrimSpace(id)
	req.Name = strings.TrimSpace(req.Name)
	if err := validateVMProfile(&req); err != nil {
		errBadRequest(w, err.Error())
		return
	}
	settings := h.state.GetSettings()
	profiles := settings.GetVMProfiles()
	found := false
	for _, p := range profiles {
		if p.ID == req.ID {
			found = true
			break
		}
	}
	if !found {
		errNotFound(w, "profile not found")
		return
	}
	if h.state.HasDB() {
		dbProfile, err := vmProfileConfigToDB(req)
		if err != nil {
			writeAppError(w, err)
			return
		}
		if err := h.state.UpdateVMProfile(dbProfile, usernameFromCtx(r)); err != nil {
			writeAppError(w, err)
			return
		}
	} else {
		newSettings := *settings
		if len(newSettings.VMProfiles) == 0 {
			newSettings.VMProfiles = state.DefaultVMProfiles()
		}
		newSettings.VMProfiles = copyVMProfiles(newSettings.VMProfiles)
		newSettings.Limits.Nodes = copyNodeLimits(settings.Limits.Nodes)
		newSettings.AddOrUpdateVMProfile(req)
		if err := h.state.SetSettings(&newSettings); err != nil {
			writeAppError(w, err)
			return
		}
	}
	writeJSON(w, req)
}

// DeleteVMProfile handles DELETE /api/v1/admin/vm-profiles/:id.
func (h *AdminMutationsHandler) DeleteVMProfile(w http.ResponseWriter, r *http.Request) {
	ps := httprouter.ParamsFromContext(r.Context())
	id := ps.ByName("id")
	if id == "" {
		errBadRequest(w, "missing profile id")
		return
	}
	settings := h.state.GetSettings()
	found := false
	for _, p := range settings.GetVMProfiles() {
		if p.ID == id {
			found = true
			break
		}
	}
	if !found {
		errNotFound(w, "profile not found")
		return
	}
	if h.state.HasDB() {
		if err := h.state.DeleteVMProfile(id, usernameFromCtx(r)); err != nil {
			writeAppError(w, err)
			return
		}
	} else {
		newSettings := *settings
		if len(newSettings.VMProfiles) == 0 {
			newSettings.VMProfiles = state.DefaultVMProfiles()
		}
		newSettings.VMProfiles = copyVMProfiles(newSettings.VMProfiles)
		newSettings.Limits.Nodes = copyNodeLimits(settings.Limits.Nodes)
		newSettings.RemoveVMProfile(id)
		if err := h.state.SetSettings(&newSettings); err != nil {
			writeAppError(w, err)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// ToggleVMProfile handles POST /api/v1/admin/vm-profiles/:id/toggle.
func (h *AdminMutationsHandler) ToggleVMProfile(w http.ResponseWriter, r *http.Request) {
	ps := httprouter.ParamsFromContext(r.Context())
	id := ps.ByName("id")
	if id == "" {
		errBadRequest(w, "missing profile id")
		return
	}
	settings := h.state.GetSettings()
	found := false
	var toggled state.VMProfileConfig
	for _, p := range settings.GetVMProfiles() {
		if p.ID == id {
			p.Enabled = !p.Enabled
			toggled = p
			found = true
			break
		}
	}
	if !found {
		errNotFound(w, "profile not found")
		return
	}
	if h.state.HasDB() {
		dbProfile, err := vmProfileConfigToDB(toggled)
		if err != nil {
			writeAppError(w, err)
			return
		}
		if err := h.state.UpdateVMProfile(dbProfile, usernameFromCtx(r)); err != nil {
			writeAppError(w, err)
			return
		}
	} else {
		newSettings := *settings
		if len(newSettings.VMProfiles) == 0 {
			newSettings.VMProfiles = state.DefaultVMProfiles()
		}
		newSettings.VMProfiles = copyVMProfiles(newSettings.VMProfiles)
		newSettings.Limits.Nodes = copyNodeLimits(settings.Limits.Nodes)
		for i, p := range newSettings.VMProfiles {
			if p.ID == id {
				newSettings.VMProfiles[i].Enabled = toggled.Enabled
				break
			}
		}
		if err := h.state.SetSettings(&newSettings); err != nil {
			writeAppError(w, err)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}
