package apiv1

import (
	"context"
	"fmt"
	"net/http"
	"path"
	"regexp"
	"strings"
	"time"

	"pvmss/cloudinit"
	apperrors "pvmss/errors"
	"pvmss/logger"
	"pvmss/proxmox"
	"pvmss/state"
)

// cloudInitSnippetResponse is the response for GET /api/v1/vms/:id/cloudinit/snippet.
type cloudInitSnippetResponse struct {
	Content  string `json:"content"`            // YAML content (empty when no snippet exists yet)
	Storage  string `json:"storage,omitempty"`  // snippets storage backing the snippet (may be empty until first save)
	Filename string `json:"filename"`           // snippet filename, e.g. pvmss-100.yml
	CICustom string `json:"cicustom,omitempty"` // current cicustom volid, if any
	Editable bool   `json:"editable"`           // true when SFTP is configured and the content can be saved; false → read-only view
}

// cloudInitSnippetRequest is the request body for PUT /api/v1/vms/:id/cloudinit/snippet.
type cloudInitSnippetRequest struct {
	Content string `json:"content"`
}

// parseCICustomUser extracts the storage and filename from the user= entry of a
// cicustom value (e.g. "user=local:snippets/pvmss-100.yml" → "local", "pvmss-100.yml").
// Returns empty strings when no user= entry is present. PVMSS only ever sets the
// user= entry, but cicustom may also carry vendor=/network=/meta= entries.
func parseCICustomUser(cicustom string) (storage, filename string) {
	for _, entry := range strings.Split(cicustom, ",") {
		entry = strings.TrimSpace(entry)
		if !strings.HasPrefix(entry, "user=") {
			continue
		}
		volid := strings.TrimPrefix(entry, "user=")
		// volid looks like "local:snippets/pvmss-100.yml"
		colon := strings.IndexByte(volid, ':')
		if colon < 0 {
			return "", ""
		}
		storage = volid[:colon]
		rest := volid[colon+1:]
		// rest looks like "snippets/pvmss-100.yml"; the filename is the base.
		filename = path.Base(rest)
		return storage, filename
	}
	return "", ""
}

// selectSnippetStorageForNode picks a snippets storage available on the node,
// preferring the provided one. Returns the default PVMSS snippets filename when
// no cicustom is set. Mirrors VMCreateHandler.selectSnippetStorage but as a
// free function so the VM details handler can reuse it without coupling.
func selectSnippetStorageForNode(ctx context.Context, client *proxmox.RestyClient, node, preferred string) (string, error) {
	storages, err := proxmox.GetSnippetsStoragesResty(ctx, client)
	if err != nil {
		return "", err
	}
	var fallback string
	for _, s := range storages {
		if s.Nodes != "" {
			found := false
			for _, n := range strings.Split(s.Nodes, ",") {
				if strings.TrimSpace(n) == node {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		if preferred != "" && s.Storage == preferred {
			return s.Storage, nil
		}
		if fallback == "" {
			fallback = s.Storage
		}
	}
	if fallback == "" {
		return "", fmt.Errorf("no snippets storage available for node %s", node)
	}
	return fallback, nil
}

// snippetFilenameForVM returns the default PVMSS snippet filename for a VM.
func snippetFilenameForVM(vmid int) string {
	return fmt.Sprintf("%s%d.yml", state.CloudInitTemplatePrefix, vmid)
}

// cloudInitSecretLineRe matches YAML lines that carry a credential
// (password, passwd, hashed_passwd, plain_text_passwd) so they can be redacted
// before a cloud-config is shown read-only. Proxmox's rendered user-data embeds
// the cipassword hash, which must never be returned to the client.
var cloudInitSecretLineRe = regexp.MustCompile(`(?im)^(\s*(?:password|passwd|hashed_passwd|plain_text_passwd)\s*:\s*).+$`)

// redactCloudInitSecrets replaces credential values in a cloud-config document
// with a placeholder, preserving the key and indentation. Used for read-only
// views so a password hash is never leaked to the UI.
func redactCloudInitSecrets(s string) string {
	if s == "" {
		return s
	}
	return cloudInitSecretLineRe.ReplaceAllString(s, "${1}<redacted>")
}

// requireSFTPEnabled writes a 400 response and returns false when SFTP snippet
// upload is not configured. The Proxmox HTTP API cannot reliably read or write
// snippets, so the custom cloud-config editor is gated on SFTP.
func (h *VMDetailsHandler) requireSFTPEnabled(w http.ResponseWriter) bool {
	if h.state.GetSettings().CloudInitSFTP.Enabled {
		return true
	}
	writeError(w, http.StatusBadRequest, "sftp_disabled",
		"SFTP is not configured; an admin must enable it in Admin > Cloud-Init to manage custom cloud-init YAML")
	return false
}

// GetVMCloudInitSnippet handles GET /api/v1/vms/:id/cloudinit/snippet.
// Returns the custom cloud-config YAML attached to the VM (read via SFTP).
// When no snippet exists yet, returns an empty content with the filename that
// will be used on save, so the editor can start from a stub.
func (h *VMDetailsHandler) GetVMCloudInitSnippet(w http.ResponseWriter, r *http.Request) {
	vmid, ok := requireVMID(w, r)
	if !ok {
		return
	}
	// GET is allowed even when SFTP is disabled: in that case we present a
	// read-only view of the rendered cloud-config via the Proxmox dump endpoint
	// (which the HTTP API can read reliably). Editing still requires SFTP.
	sftpEnabled := h.state.GetSettings().CloudInitSFTP.Enabled
	if h.isOffline() {
		errOffline(w)
		return
	}

	username := usernameFromCtx(r)
	isAdmin := isAdminFromCtx(r)

	envCfg := h.state.GetEnvConfig()
	client, err := proxmox.MakeRestyClientFromEnvConfig(envCfg, 30*time.Second)
	if err != nil {
		writeAppError(w, err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	if !ownsVM(ctx, client, username, isAdmin, vmid) {
		writeError(w, http.StatusNotFound, "not_found", "VM not found")
		return
	}

	node, err := resolveNode(ctx, client, vmid)
	if err != nil || node == "" {
		writeError(w, http.StatusNotFound, "not_found", "VM not found")
		return
	}

	cfg, err := proxmox.GetVMConfigResty(ctx, client, node, vmid)
	if err != nil {
		writeAppError(w, err)
		return
	}

	cicustom := ""
	if v, ok := cfg["cicustom"].(string); ok {
		cicustom = v
	}
	storage, filename := parseCICustomUser(cicustom)
	if filename == "" {
		filename = snippetFilenameForVM(vmid)
	}

	content := ""
	if sftpEnabled {
		// SFTP configured: read the editable per-VM snippet file directly.
		if storage != "" {
			sftpCfg := h.state.GetSettings().CloudInitSFTP
			if content, err = proxmox.ReadSnippetFileSFTP(ctx, sftpCfg, filename); err != nil {
				// Reading is best-effort: a missing/unreadable snippet should not
				// block the editor, since the user may be recovering from a failed
				// upload during creation.
				logger.Get().Warn().Err(err).Int("vmid", vmid).Str("filename", filename).
					Msg("api/v1: failed to read cloud-init snippet via SFTP, returning empty content")
				content = ""
			}
		}
	} else if storage != "" {
		// SFTP not configured but a custom snippet is attached (cicustom set):
		// the Proxmox dump returns that custom user snippet verbatim, so we can
		// present it read-only. We deliberately do NOT dump when there is no
		// cicustom — in that case the dump returns Proxmox's generated user-data
		// (which contains the cipassword hash and is not a "custom" config), so
		// we return empty and let the UI explain there is no custom config.
		if content, err = proxmox.GetVMCloudInitDumpResty(ctx, client, node, vmid, "user"); err != nil {
			logger.Get().Warn().Err(err).Int("vmid", vmid).
				Msg("api/v1: failed to dump cloud-init user-data, returning empty content")
			content = ""
		}
		content = redactCloudInitSecrets(content)
	}

	writeJSON(w, cloudInitSnippetResponse{
		Content:  content,
		Storage:  storage,
		Filename: filename,
		CICustom: cicustom,
		Editable: sftpEnabled,
	})
}

// UpdateVMCloudInitSnippet handles PUT /api/v1/vms/:id/cloudinit/snippet.
// Validates the YAML, (re)uploads the snippet via SFTP, and sets cicustom on
// the VM when it is not already pointing at the snippet.
func (h *VMDetailsHandler) UpdateVMCloudInitSnippet(w http.ResponseWriter, r *http.Request) {
	vmid, ok := requireVMID(w, r)
	if !ok {
		return
	}
	if !h.requireSFTPEnabled(w) {
		return
	}

	var req cloudInitSnippetRequest
	if !decodeBody(w, r, &req) {
		return
	}

	// Validate the YAML before any Proxmox/SFTP interaction. Require the
	// #cloud-config header so users don't silently save an unusable snippet.
	// Wrap the cloudinit package error in an apperrors.ValidationError so
	// writeAppError maps it to HTTP 400 (the cloudinit validator defines its
	// own error type that writeAppError does not recognise).
	if err := cloudinit.ValidateCloudInitYAMLStrict(req.Content); err != nil {
		writeAppError(w, apperrors.ValidationErr("content", req.Content, err.Error()))
		return
	}

	if h.isOffline() {
		errOffline(w)
		return
	}

	username := usernameFromCtx(r)
	isAdmin := isAdminFromCtx(r)

	envCfg := h.state.GetEnvConfig()
	client, err := proxmox.MakeRestyClientFromEnvConfig(envCfg, 30*time.Second)
	if err != nil {
		writeAppError(w, err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if !ownsVM(ctx, client, username, isAdmin, vmid) {
		writeError(w, http.StatusNotFound, "not_found", "VM not found")
		return
	}

	node, err := resolveNode(ctx, client, vmid)
	if err != nil || node == "" {
		writeError(w, http.StatusNotFound, "not_found", "VM not found")
		return
	}

	cfg, err := proxmox.GetVMConfigResty(ctx, client, node, vmid)
	if err != nil {
		writeAppError(w, err)
		return
	}

	existingCICustom := ""
	if v, ok := cfg["cicustom"].(string); ok {
		existingCICustom = v
	}
	storage, filename := parseCICustomUser(existingCICustom)
	if filename == "" {
		filename = snippetFilenameForVM(vmid)
	}
	if storage == "" {
		// No cicustom yet: pick a snippets storage for this node.
		storage, err = selectSnippetStorageForNode(ctx, client, node, "")
		if err != nil {
			writeAppError(w, err)
			return
		}
	}

	sftpCfg := h.state.GetSettings().CloudInitSFTP
	if err := proxmox.UploadSnippetFileSFTP(ctx, sftpCfg, filename, req.Content); err != nil {
		writeAppError(w, err)
		return
	}

	// If the VM didn't already reference this snippet via cicustom, set it now.
	desiredCICustom := fmt.Sprintf("user=%s:snippets/%s", storage, filename)
	if existingCICustom != desiredCICustom {
		if err := proxmox.UpdateVMConfigResty(ctx, client, node, vmid, map[string]string{
			"cicustom": desiredCICustom,
		}); err != nil {
			writeAppError(w, err)
			return
		}
		proxmox.InvalidateVMCache(node)
	}

	logger.Get().Info().
		Int("vmid", vmid).
		Str("node", node).
		Str("storage", storage).
		Str("filename", filename).
		Msg("api/v1: cloud-init snippet updated")

	writeJSON(w, cloudInitSnippetResponse{
		Content:  req.Content,
		Storage:  storage,
		Filename: filename,
		CICustom: desiredCICustom,
		Editable: true,
	})
}
