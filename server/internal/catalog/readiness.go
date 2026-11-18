package catalog

import "slices"

// Reasons an approved node cannot host a new VM, as stable codes the UI
// translates.
const (
	MissingStorage       = "storage"
	MissingBridge        = "bridge"
	MissingISOOrTemplate = "iso_or_template"
)

// NodeReadiness lists what node lacks to host a new VM: an approved storage,
// an approved bridge, and an install source (an approved ISO or a template on
// that node). An empty result means the node is ready. It is the one readiness
// rule shared by the create picker and the admin dashboard.
func NodeReadiness(resources Resources, templates []Template, node string) []string {
	var missing []string

	if !slices.ContainsFunc(resources.Storages, func(s Storage) bool { return s.Node == node }) {
		missing = append(missing, MissingStorage)
	}

	if !slices.ContainsFunc(resources.Bridges, func(b Bridge) bool { return b.Node == node }) {
		missing = append(missing, MissingBridge)
	}

	hasISO := slices.ContainsFunc(resources.ISOs, func(i ISO) bool { return i.Node == node })
	hasTemplate := slices.ContainsFunc(templates, func(t Template) bool { return t.Node == node })

	if !hasISO && !hasTemplate {
		missing = append(missing, MissingISOOrTemplate)
	}

	return missing
}
