package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"pvmss/server/internal/catalog"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/policy"
	"pvmss/server/internal/vm"
)

// createTarget bundles the per-cluster capabilities the create path needs,
// resolved from the request's own cluster (never the default client).
type createTarget struct {
	clusterName string
	creator     cluster.Creator
	pusher      vm.CloudInitPusher
	writer      vm.HardwareUpdater
	freeSpace   vm.FreeSpaceChecker
	snippets    vm.SnippetStorageFinder
	templates   vm.TemplateReader
}

// resolveCreateTarget resolves the effective cluster name from req.Cluster
// (defaulting the same way ResolveClusterParam does for the catalog route)
// plus that cluster's own Creator, CloudInitPusher, HardwareUpdater, and
// SnippetStorageFinder - without this, VM creation ran through the default
// cluster's client regardless of which cluster the request named. The
// HardwareUpdater is needed for post-clone configuration; the
// SnippetStorageFinder for the plan-time snippet storage resolution.
func (h *VMCreate) resolveCreateTarget(w http.ResponseWriter, requestedCluster string) (createTarget, bool) {
	clusterName, err := ResolveClusterValue(requestedCluster, h.clients)
	if err != nil {
		code, message := clusterParamError(err)
		h.writeCreateError(w, http.StatusBadRequest, code, message)

		return createTarget{}, false
	}

	if h.clients == nil {
		writer, _ := h.creator.(vm.HardwareUpdater)
		freeSpace, _ := h.creator.(vm.FreeSpaceChecker)
		snippets, _ := h.creator.(vm.SnippetStorageFinder)
		templates, _ := h.creator.(vm.TemplateReader)

		return createTarget{clusterName: clusterName, creator: h.creator, pusher: h.pusher, writer: writer, freeSpace: freeSpace, snippets: snippets, templates: templates}, true
	}

	client, err := h.clients.Client(clusterName)
	if err != nil {
		h.writeCreateError(w, http.StatusNotFound, "not_found", msgClusterNotFound)
		return createTarget{}, false
	}

	creator, ok := client.(cluster.Creator)
	if !ok {
		SetError(w, errors.New("cluster client does not implement Creator"))
		h.writeCreateError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return createTarget{}, false
	}

	pusher, ok := client.(vm.CloudInitPusher)
	if !ok {
		SetError(w, errors.New("cluster client does not implement CloudInitPusher"))
		h.writeCreateError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return createTarget{}, false
	}

	writer, ok := client.(vm.HardwareUpdater)
	if !ok {
		SetError(w, errors.New("cluster client does not implement HardwareUpdater"))
		h.writeCreateError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return createTarget{}, false
	}

	freeSpace, ok := client.(vm.FreeSpaceChecker)
	if !ok {
		SetError(w, errors.New("cluster client does not implement FreeSpaceChecker"))
		h.writeCreateError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return createTarget{}, false
	}

	// Optional capability: a client without FindSnippetStorage only blocks
	// cloud-init template requests (planCreate refuses before VMID
	// allocation), not plain ISO creations.
	snippets, _ := client.(vm.SnippetStorageFinder)

	// Optional capability: the clone-time freshness backstop. A client
	// without TemplateByVMID skips the backstop.
	templates, _ := client.(vm.TemplateReader)

	return createTarget{clusterName: clusterName, creator: creator, pusher: pusher, writer: writer, freeSpace: freeSpace, snippets: snippets, templates: templates}, true
}

func (h *VMCreate) clientFor(clusterName string) (cluster.Client, error) {
	if h.clients != nil {
		return h.clients.Client(clusterName)
	}

	if h.client == nil {
		return nil, cluster.ErrClusterNotFound
	}

	return h.client, nil
}

func vmCapableStorage(storage catalog.Storage, available []cluster.Storage) (cluster.Storage, bool) {
	for _, candidate := range available {
		if candidate.Name == storage.Name && candidate.Node == storage.Node && cluster.IsVMCapableStorage(candidate) {
			return candidate, true
		}
	}

	return cluster.Storage{}, false
}

// isoDiscoveryKey is a composite map key for ISOs, avoiding string-concat
// collisions when a storage or file contains ":".
type isoDiscoveryKey struct {
	Node    string
	Storage string
	File    string
}

// catalogBridgeDTOs dedupes by (name, node) - the same bridge name can be
// approved on more than one node, and each is a distinct, independently
// selectable option (bridge approval is per-node, like storage). live carries
// the cluster's current network config, which is where the description
// (Proxmox "comments" field) actually lives - catalog_bridges only stores
// the approval, not the comment. Bridges absent from live (orphan approvals
// whose bridge Proxmox no longer reports) are dropped so users never see a
// bridge they cannot actually use.
func catalogBridgeDTOs(bridges []catalog.Bridge, live []cluster.Bridge) []catalogBridgeDTO {
	type key struct{ name, node string }

	commentByKey := make(map[key]string, len(live))

	liveByKey := make(map[key]bool, len(live))
	for _, bridge := range live {
		commentByKey[key{bridge.Name, bridge.Node}] = bridge.Comment
		liveByKey[key{bridge.Name, bridge.Node}] = true
	}

	out := make([]catalogBridgeDTO, 0, len(bridges))
	seen := make(map[key]struct{}, len(bridges))

	for _, bridge := range bridges {
		k := key{bridge.Name, bridge.Node}
		if _, exists := seen[k]; exists {
			continue
		}

		if !liveByKey[k] {
			continue
		}

		seen[k] = struct{}{}
		out = append(out, catalogBridgeDTO{Name: bridge.Name, Node: bridge.Node, Comment: commentByKey[k]})
	}

	return out
}

// writeCreateFailure maps vm.Create's sentinel errors to the contract's
// status codes and error codes.
func (h *VMCreate) writeCreateFailure(w http.ResponseWriter, err error) {
	if status, code, message, ok := mapCreateError(err); ok {
		if code == "cluster_error" {
			SetErrorMsg(w, "cluster create failed", err)
		}

		h.writeCreateError(w, status, code, message)

		return
	}

	SetErrorMsg(w, "vm create failed", err)
	h.writeCreateError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)
}

// createErrorMapping pairs a sentinel error with its HTTP status, error code,
// and message. A nil message means "use err.Error()" (the sentinel carries a
// dynamic detail string).
type createErrorMapping struct {
	err     error
	status  int
	code    string
	message string // empty → err.Error()
}

// createErrorMappings is the table writeCreateFailure consults. Order matters
// only for errors.Is precedence, which is identity-based here.
var createErrorMappings = []createErrorMapping{
	{vm.ErrAdminCannotCreate, http.StatusForbidden, "admin_cannot_create", "administrators cannot create VMs"},
	{vm.ErrNoPool, http.StatusForbidden, "no_pool", "this account cannot own VMs"},
	{policy.ErrQuotaExceeded, http.StatusBadRequest, "quota_exceeded", ""},
	{policy.ErrGabaritExceeded, http.StatusBadRequest, "gabarit_exceeded", ""},
	{policy.ErrNodeCapacityExceeded, http.StatusBadRequest, "capacity_exceeded", ""},
	{vm.ErrInvalidName, http.StatusBadRequest, "invalid_name", "name must be a valid hostname (lowercase alphanumeric and hyphen, no leading/trailing hyphen, max 63 chars)"},
	{vm.ErrNameTaken, http.StatusBadRequest, "name_taken", ""},
	{vm.ErrOutOfRange, http.StatusBadRequest, "out_of_range", ""},
	{vm.ErrNotApproved, http.StatusBadRequest, "not_approved", ""},
	{vm.ErrInvalidSource, http.StatusBadRequest, "invalid_source", ""},
	{vm.ErrInvalidRequest, http.StatusBadRequest, codeInvalidRequest, ""},
	{vm.ErrDiskReduction, http.StatusBadRequest, "disk_reduction", ""},
	{vm.ErrDiskBelowImage, http.StatusBadRequest, "disk_below_image", ""},
	{vm.ErrInsufficientDiskSpace, http.StatusBadRequest, "insufficient_disk_space", ""},
	{vm.ErrCloudInitWriteUnavailable, http.StatusConflict, "cloudinit_write_unavailable", "cloud-init documents are not enabled on this cluster (Infrastructure > Clusters: snippet storage and SSH publishing)"},
	{vm.ErrCloudInitNotPublished, http.StatusConflict, "cloudinit_not_published", ""},
	{vm.ErrNoSnippetStorage, http.StatusBadRequest, "no_snippet_storage", ""},
	// cluster_error passes the full error chain (empty message → err.Error()):
	// the Proxmox rejection text ("'import-from' requires special syntax", "has wrong type 'iso'",
	// ...) is the only way to diagnose a 502 from the
	// browser, and the frontend surfaces it after the localized prefix.
	{vm.ErrClusterCreate, http.StatusBadGateway, "cluster_error", ""},
}

// mapCreateError returns the HTTP status, code, and message for a known
// sentinel error, or (0, "", "", false) for an unrecognized error.
func mapCreateError(err error) (int, string, string, bool) {
	for _, m := range createErrorMappings {
		if errors.Is(err, m.err) {
			msg := m.message
			if msg == "" {
				msg = err.Error()
			}

			return m.status, m.code, msg, true
		}
	}

	return 0, "", "", false
}

func (h *VMCreate) writeCreateJSON(w http.ResponseWriter, status int, value any) {
	body, err := json.Marshal(value)
	if err != nil {
		SetErrorMsg(w, "failed to marshal response", err)
		h.writeCreateError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	if err := writeJSON(w, status, body); err != nil {
		h.log.Warn("failed to write response", "component", "httpapi", "error", err)
	}
}

func (h *VMCreate) writeCreateError(w http.ResponseWriter, status int, code, message string) {
	if err := writeClusterError(w, status, code, message); err != nil {
		h.log.Warn("failed to write error response", "component", "httpapi", "code", code, "error", err)
	}
}
