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
				uploaded := false
				if settings.CloudInitSFTP.Enabled {
					if err := proxmox.UploadSnippetFileSFTP(ctx, settings.CloudInitSFTP, filename, template.YAMLContent); err == nil {
						uploaded = true
					}
				}
				if !uploaded {
					if err := proxmox.UploadSnippetFileResty(ctx, client, node, snippetStorage, filename, template.YAMLContent); err == nil {
						uploaded = true
					}
				}
				if uploaded {
					ciParams.CICustom = fmt.Sprintf("user=%s:snippets/%s", snippetStorage, filename)
				} else {
					warning = "upload-failed"
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
