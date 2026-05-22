package apiv1

import (
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"

	"pvmss/proxmox"
)

var tagNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,50}$`)

// hexColorRegex validates 6-digit hex colors (with optional leading #).
var hexColorRegex = regexp.MustCompile(`^#?[0-9a-fA-F]{6}$`)

// ListTags handles GET /api/v1/admin/tags.
func (h *AdminMutationsHandler) ListTags(w http.ResponseWriter, r *http.Request) {
	tags := h.state.GetTags()
	if tags == nil {
		tags = []string{}
	}

	tagCounts := make(map[string]int, len(tags))
	tagColors := map[string]proxmox.TagColor{}
	if !h.state.IsOfflineMode() {
		snap := h.state.GetProxmoxSnapshot()
		if snap != nil {
			for _, vm := range snap.VMs {
				for _, tag := range strings.Split(vm.Tags, ";") {
					tag = strings.TrimSpace(tag)
					if tag != "" {
						tagCounts[tag]++
					}
				}
			}
		}
		if restyClient, err := proxmox.MakeRestyClientFromEnv(5 * time.Second); err == nil {
			if colors, cErr := proxmox.GetTagColorsResty(r.Context(), restyClient); cErr == nil {
				tagColors = colors
			}
		}
	}

	result := make([]AdminTagResponse, 0, len(tags))
	for _, tag := range tags {
		entry := AdminTagResponse{
			Name:    tag,
			VMCount: tagCounts[tag],
		}
		if color, ok := tagColors[tag]; ok {
			entry.Color = color.Background
			entry.TextColor = color.Text
			entry.FromProxmox = true
		}
		result = append(result, entry)
	}
	writeJSON(w, result)
}

// CreateTag handles POST /api/v1/admin/tags.
func (h *AdminMutationsHandler) CreateTag(w http.ResponseWriter, r *http.Request) {
	var req CreateTagRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Name == "" {
		errBadRequest(w, "name is required")
		return
	}
	if !tagNameRegex.MatchString(req.Name) {
		errBadRequest(w, "invalid tag name: use only letters, digits, hyphens, underscores (max 50 chars)")
		return
	}

	settings := h.state.GetSettings()
	for _, t := range settings.Tags {
		if t == req.Name {
			errBadRequest(w, "tag already exists")
			return
		}
	}

	newTags := make([]string, len(settings.Tags), len(settings.Tags)+1)
	copy(newTags, settings.Tags)
	newTags = append(newTags, req.Name)
	if h.state.HasDB() {
		if err := h.state.SetTags(newTags, usernameFromCtx(r)); err != nil {
			writeAppError(w, err)
			return
		}
	} else {
		newSettings := *settings
		newSettings.Tags = newTags
		newSettings.Limits.Nodes = copyNodeLimits(settings.Limits.Nodes)
		if err := h.state.SetSettings(&newSettings); err != nil {
			writeAppError(w, err)
			return
		}
	}
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, AdminTagResponse{Name: req.Name})
}

// SetTagColor handles PUT /api/v1/admin/tags/:name/color.
// Updates the cluster-wide `tag-style` color-map in Proxmox datacenter options.
// An empty color in the body removes the entry.
func (h *AdminMutationsHandler) SetTagColor(w http.ResponseWriter, r *http.Request) {
	if h.state.IsOfflineMode() {
		errBadRequest(w, "tag colors require an online Proxmox connection")
		return
	}
	ps := httprouter.ParamsFromContext(r.Context())
	name := ps.ByName("name")
	if name == "" {
		errBadRequest(w, "missing tag name")
		return
	}
	settings := h.state.GetSettings()
	known := false
	for _, t := range settings.Tags {
		if t == name {
			known = true
			break
		}
	}
	if !known {
		errBadRequest(w, "tag not found")
		return
	}

	var req SetTagColorRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Color != "" && !hexColorRegex.MatchString(req.Color) {
		errBadRequest(w, "color must be a 6-digit hex value")
		return
	}
	if req.TextColor != "" && !hexColorRegex.MatchString(req.TextColor) {
		errBadRequest(w, "text_color must be a 6-digit hex value")
		return
	}
	if req.Color == "" && req.TextColor != "" {
		errBadRequest(w, "text_color requires color")
		return
	}

	restyClient, err := proxmox.MakeRestyClientFromEnv(10 * time.Second)
	if err != nil {
		writeAppError(w, err)
		return
	}
	if err := proxmox.SetTagColorResty(r.Context(), restyClient, name, req.Color, req.TextColor); err != nil {
		writeAppError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DeleteTag handles DELETE /api/v1/admin/tags/:name.
func (h *AdminMutationsHandler) DeleteTag(w http.ResponseWriter, r *http.Request) {
	ps := httprouter.ParamsFromContext(r.Context())
	name := ps.ByName("name")
	if name == "" {
		errBadRequest(w, "missing tag name")
		return
	}
	if strings.EqualFold(name, "pvmss") {
		errBadRequest(w, "cannot delete the default 'pvmss' tag")
		return
	}

	settings := h.state.GetSettings()
	newTags := make([]string, 0, len(settings.Tags))
	found := false
	for _, t := range settings.Tags {
		if t == name {
			found = true
			continue
		}
		newTags = append(newTags, t)
	}
	if !found {
		errBadRequest(w, "tag not found")
		return
	}
	if h.state.HasDB() {
		if err := h.state.SetTags(newTags, usernameFromCtx(r)); err != nil {
			writeAppError(w, err)
			return
		}
	} else {
		newSettings := *settings
		newSettings.Tags = newTags
		newSettings.Limits.Nodes = copyNodeLimits(settings.Limits.Nodes)
		if err := h.state.SetSettings(&newSettings); err != nil {
			writeAppError(w, err)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}
