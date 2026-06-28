package apiv1

import (
	"context"
	"fmt"
	"strings"

	"pvmss/logger"
	"pvmss/proxmox"
	"pvmss/state"
)

// applyCloudInit applies cloud-init configuration to a newly created VM.
// Returns a stable warning code (empty string when everything succeeded).
//
// Warning codes:
//   - "upload-failed-sftp": SFTP was enabled but the snippet upload failed.
//   - "upload-failed-api":  SFTP was disabled (or also failed) and the Proxmox
//     HTTP API upload failed. This is the common case: the /upload endpoint
//     returns 400 for content=snippets in many Proxmox versions and SFTP is
//     the supported workaround.
//   - "no-snippets-storage": no storage on the node supports the snippets
//     content type.
//   - "cloud-init-config-failed": applying the cloud-init config to the VM
//     failed.
func (h *VMCreateHandler) applyCloudInit(ctx context.Context, client *proxmox.RestyClient, node string, vmid int, storage string, ci *VMCreateCloudInit, settings *state.AppSettings) string {
	warning := ""

	ciParams := proxmox.CloudInitParams{CIUser: ci.User}
	if ci.Password != "" {
		ciParams.CIPassword = ci.Password
	}
	if ci.SSHKeys != "" {
		ciParams.SSHKeys = ci.SSHKeys
	}
	if ci.IPConfig == "static" && ci.IP != "" {
		ipConfig := "ip=" + ci.IP
		if ci.Gateway != "" {
			ipConfig += ",gw=" + ci.Gateway
		}
		ciParams.IPConfig0 = ipConfig
	} else {
		ciParams.IPConfig0 = "ip=dhcp"
	}
	if ci.DNS != "" {
		ciParams.Nameserver = ci.DNS
	}

	if ci.TemplateID != "" {
		template := settings.GetCloudInitTemplateByID(ci.TemplateID)
		if template != nil && strings.TrimSpace(template.YAMLContent) != "" {
			snippetStorage, err := h.selectSnippetStorage(ctx, client, node, template.Storage)
			if err == nil {
				filename := fmt.Sprintf("%s%d.yml", state.CloudInitTemplatePrefix, vmid)
				cicustom, snippetWarn := h.uploadSnippet(ctx, client, node, snippetStorage, filename, template.YAMLContent, settings)
				if cicustom != "" {
					ciParams.CICustom = cicustom
				}
				if snippetWarn != "" {
					warning = snippetWarn
				}
			} else {
				warning = "no-snippets-storage"
			}
		}
	}

	if err := proxmox.EnsureCloudInitDriveResty(ctx, client, node, vmid, storage); err != nil {
		logger.Get().Error().Err(err).Int("vmid", vmid).Msg("api/v1: failed to ensure cloud-init drive")
	}

	if err := proxmox.UpdateVMCloudInitConfigResty(ctx, client, node, vmid, ciParams); err != nil {
		logger.Get().Error().Err(err).Int("vmid", vmid).Msg("api/v1: failed to apply cloud-init config")
		return "cloud-init-config-failed"
	}

	return warning
}

// uploadSnippet uploads a cloud-init snippet via SFTP (preferred) or the
// Proxmox HTTP API (fallback). Returns the cicustom volid on success (empty
// string on failure) and a stable warning code (empty string on success).
//
// The HTTP API fallback returns 400 "bad request" for content=snippets on many
// Proxmox versions; in that case users must configure SFTP — see
// backend/docs/cloud-init.{en,fr}.md. We therefore distinguish SFTP failures
// from API fallback failures so the UI can guide the user.
func (h *VMCreateHandler) uploadSnippet(ctx context.Context, client *proxmox.RestyClient, node, snippetStorage, filename, content string, settings *state.AppSettings) (string, string) {
	sftpEnabled := settings.CloudInitSFTP.Enabled

	if sftpEnabled {
		if err := h.uploadSnippetSFTP(ctx, settings.CloudInitSFTP, filename, content); err == nil {
			return fmt.Sprintf("user=%s:snippets/%s", snippetStorage, filename), ""
		} else {
			logger.Get().Warn().
				Err(err).
				Str("node", node).
				Str("storage", snippetStorage).
				Str("filename", filename).
				Msg("api/v1: SFTP snippet upload failed, falling back to HTTP API")
		}
	}

	if err := h.uploadSnippetAPI(ctx, client, node, snippetStorage, filename, content); err == nil {
		return fmt.Sprintf("user=%s:snippets/%s", snippetStorage, filename), ""
	} else {
		logger.Get().Warn().
			Err(err).
			Str("node", node).
			Str("storage", snippetStorage).
			Str("filename", filename).
			Bool("sftp_enabled", sftpEnabled).
			Msg("api/v1: HTTP API snippet upload failed")
	}

	if sftpEnabled {
		return "", "upload-failed-sftp"
	}
	return "", "upload-failed-api"
}

// selectSnippetStorage picks a snippets storage for the given node.
func (h *VMCreateHandler) selectSnippetStorage(ctx context.Context, client *proxmox.RestyClient, node string, preferred string) (string, error) {
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

// isTPMCompatibleStorage checks if the storage type supports TPM.
func isTPMCompatibleStorage(storageType string) bool {
	compatible := map[string]bool{
		"iscsi": true, "lvm": true, "lvmthin": true, "rbd": true, "zfs": true,
		"cephfs": true, "cifs": true, "dir": true, "nfs": true,
	}
	return compatible[storageType]
}
