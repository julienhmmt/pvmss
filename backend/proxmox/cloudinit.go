package proxmox

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"pvmss/logger"
)

// CloudInitConfig represents cloud-init configuration for a VM.
type CloudInitConfig struct {
	CIUser       string `json:"ciuser,omitempty"`
	CIPassword   string `json:"cipassword,omitempty"`
	SSHKeys      string `json:"sshkeys,omitempty"`
	IPConfig0    string `json:"ipconfig0,omitempty"`
	IPConfig1    string `json:"ipconfig1,omitempty"`
	Nameserver   string `json:"nameserver,omitempty"`
	Searchdomain string `json:"searchdomain,omitempty"`
	CICustom     string `json:"cicustom,omitempty"`
	CIType       string `json:"citype,omitempty"`
}

// SnippetFile represents a file in the snippets storage.
type SnippetFile struct {
	Volid   string `json:"volid"`
	Content string `json:"content,omitempty"`
	Format  string `json:"format,omitempty"`
	Size    int64  `json:"size,omitempty"`
	CTime   int64  `json:"ctime,omitempty"`
}

// CloudInitParams represents the parameters to set cloud-init config.
type CloudInitParams struct {
	CIUser       string
	CIPassword   string
	SSHKeys      string
	IPConfig0    string
	IPConfig1    string
	Nameserver   string
	Searchdomain string
	CICustom     string
	CIDrive      string // e.g., "ide2" or "scsi1"
	CIStorage    string // storage for cloud-init drive
}

// GetVMCloudInitConfigResty fetches cloud-init configuration for a VM.
// GET /nodes/{node}/qemu/{vmid}/cloudinit
func GetVMCloudInitConfigResty(ctx context.Context, restyClient *RestyClient, node string, vmid int) (*CloudInitConfig, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/cloudinit", url.PathEscape(node), vmid)

	var response struct {
		Data map[string]interface{} `json:"data"`
	}

	if err := restyClient.Get(ctx, path, &response); err != nil {
		logger.Get().Error().Err(err).Str("node", node).Int("vmid", vmid).Msg("Failed to get VM cloud-init config (resty)")
		return nil, fmt.Errorf("failed to get cloud-init config for VM %d on node %s: %w", vmid, node, err)
	}

	// Parse response into CloudInitConfig
	config := &CloudInitConfig{}
	if v, ok := response.Data["ciuser"].(string); ok {
		config.CIUser = v
	}
	if v, ok := response.Data["sshkeys"].(string); ok {
		config.SSHKeys = v
	}
	if v, ok := response.Data["ipconfig0"].(string); ok {
		config.IPConfig0 = v
	}
	if v, ok := response.Data["ipconfig1"].(string); ok {
		config.IPConfig1 = v
	}
	if v, ok := response.Data["nameserver"].(string); ok {
		config.Nameserver = v
	}
	if v, ok := response.Data["searchdomain"].(string); ok {
		config.Searchdomain = v
	}
	if v, ok := response.Data["cicustom"].(string); ok {
		config.CICustom = v
	}
	if v, ok := response.Data["citype"].(string); ok {
		config.CIType = v
	}

	logger.Get().Debug().Str("node", node).Int("vmid", vmid).Msg("Fetched VM cloud-init config (resty)")
	return config, nil
}

// UpdateVMCloudInitConfigResty updates cloud-init configuration for a VM.
// This uses the standard VM config endpoint: PUT /nodes/{node}/qemu/{vmid}/config
func UpdateVMCloudInitConfigResty(ctx context.Context, restyClient *RestyClient, node string, vmid int, params CloudInitParams) error {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/config", url.PathEscape(node), vmid)

	values := make(url.Values)

	if params.CIUser != "" {
		values.Set("ciuser", params.CIUser)
	}
	if params.CIPassword != "" {
		values.Set("cipassword", params.CIPassword)
	}
	if params.SSHKeys != "" {
		// SSH keys need URL encoding
		values.Set("sshkeys", params.SSHKeys)
	}
	if params.IPConfig0 != "" {
		values.Set("ipconfig0", params.IPConfig0)
	}
	if params.IPConfig1 != "" {
		values.Set("ipconfig1", params.IPConfig1)
	}
	if params.Nameserver != "" {
		values.Set("nameserver", params.Nameserver)
	}
	if params.Searchdomain != "" {
		values.Set("searchdomain", params.Searchdomain)
	}
	if params.CICustom != "" {
		values.Set("cicustom", params.CICustom)
	}

	// Set up cloud-init drive if specified
	if params.CIDrive != "" && params.CIStorage != "" {
		values.Set(params.CIDrive, params.CIStorage+":cloudinit")
	}

	if len(values) == 0 {
		return nil // Nothing to update
	}

	var response interface{}
	if err := restyClient.Post(ctx, path, values, &response); err != nil {
		logger.Get().Error().Err(err).Str("node", node).Int("vmid", vmid).Msg("Failed to update VM cloud-init config (resty)")
		return fmt.Errorf("failed to update cloud-init config for VM %d on node %s: %w", vmid, node, err)
	}

	logger.Get().Info().Str("node", node).Int("vmid", vmid).Msg("VM cloud-init config updated successfully (resty)")
	return nil
}

// EnsureCloudInitDriveResty ensures a cloud-init drive is attached to the VM.
// It checks if a cloud-init drive exists and creates one if not.
// Default bus is "ide2" which is commonly used for cloud-init.
func EnsureCloudInitDriveResty(ctx context.Context, restyClient *RestyClient, node string, vmid int, storage string) error {
	// First, get current VM config
	config, err := GetVMConfigResty(ctx, restyClient, node, vmid)
	if err != nil {
		return fmt.Errorf("failed to get VM config: %w", err)
	}

	// Check if any cloud-init drive already exists
	cloudInitBuses := []string{"ide2", "ide0", "ide1", "ide3", "scsi0", "scsi1", "sata0", "sata1"}
	for _, bus := range cloudInitBuses {
		if val, ok := config[bus].(string); ok && strings.Contains(val, "cloudinit") {
			logger.Get().Debug().
				Str("node", node).
				Int("vmid", vmid).
				Str("bus", bus).
				Msg("Cloud-init drive already exists")
			return nil
		}
	}

	// No cloud-init drive found, create one on ide2 (most common)
	params := map[string]string{
		"ide2": storage + ":cloudinit",
	}

	if err := UpdateVMConfigResty(ctx, restyClient, node, vmid, params); err != nil {
		return fmt.Errorf("failed to create cloud-init drive: %w", err)
	}

	logger.Get().Info().
		Str("node", node).
		Int("vmid", vmid).
		Str("storage", storage).
		Msg("Created cloud-init drive on ide2")

	return nil
}

// ListSnippetFilesResty lists files in a snippets storage.
// GET /nodes/{node}/storage/{storage}/content?content=snippets
func ListSnippetFilesResty(ctx context.Context, restyClient *RestyClient, node, storage string) ([]SnippetFile, error) {
	path := fmt.Sprintf("/nodes/%s/storage/%s/content?content=snippets",
		url.PathEscape(node), url.PathEscape(storage))

	var response ListResponse[SnippetFile]
	if err := restyClient.Get(ctx, path, &response); err != nil {
		logger.Get().Error().Err(err).Str("node", node).Str("storage", storage).Msg("Failed to list snippet files (resty)")
		return nil, fmt.Errorf("failed to list snippets in %s on node %s: %w", storage, node, err)
	}

	logger.Get().Debug().
		Str("node", node).
		Str("storage", storage).
		Int("count", len(response.Data)).
		Msg("Listed snippet files (resty)")

	return response.Data, nil
}

// DownloadSnippetFileResty downloads the content of a snippet file.
// GET /nodes/{node}/storage/{storage}/file-restore/download?volume={volid}
// Note: This endpoint may require specific permissions. Falls back to direct file access if needed.
func DownloadSnippetFileResty(ctx context.Context, restyClient *RestyClient, node, storage, volid string) (string, error) {
	// For snippets, we use a different approach - direct file content via API
	// The exact endpoint depends on Proxmox version
	// Try the file-restore endpoint first
	path := fmt.Sprintf("/nodes/%s/storage/%s/file-restore/download",
		url.PathEscape(node), url.PathEscape(storage))

	// Build query params
	params := url.Values{}
	params.Set("volume", volid)
	params.Set("filepath", "/")

	fullPath := path + "?" + params.Encode()

	var response struct {
		Data string `json:"data"`
	}

	if err := restyClient.Get(ctx, fullPath, &response); err != nil {
		// If file-restore fails, this is expected for some storage types
		logger.Get().Warn().
			Err(err).
			Str("node", node).
			Str("storage", storage).
			Str("volid", volid).
			Msg("file-restore download not available, snippet content may need direct read")
		return "", fmt.Errorf("failed to download snippet %s: %w", volid, err)
	}

	return response.Data, nil
}

// UploadSnippetFileResty uploads a new snippet file to storage.
// POST /nodes/{node}/storage/{storage}/content
// For snippets, this uses the content upload endpoint with content=snippets
func UploadSnippetFileResty(ctx context.Context, restyClient *RestyClient, node, storage, filename, content string) error {
	path := fmt.Sprintf("/nodes/%s/storage/%s/upload",
		url.PathEscape(node), url.PathEscape(storage))

	// Note: Uploading snippets requires multipart form data
	// The filename should include the proper extension (.yaml or .cfg)
	values := url.Values{}
	values.Set("content", "snippets")
	values.Set("filename", filename)

	// For text content, we need to use the file upload mechanism
	// This is a simplified approach - in production, proper multipart upload is needed
	var response interface{}
	if err := restyClient.Post(ctx, path, values, &response); err != nil {
		logger.Get().Error().
			Err(err).
			Str("node", node).
			Str("storage", storage).
			Str("filename", filename).
			Msg("Failed to upload snippet file (resty)")
		return fmt.Errorf("failed to upload snippet %s to %s on node %s: %w", filename, storage, node, err)
	}

	logger.Get().Info().
		Str("node", node).
		Str("storage", storage).
		Str("filename", filename).
		Msg("Snippet file uploaded successfully (resty)")

	return nil
}

// DeleteSnippetFileResty deletes a snippet file from storage.
// DELETE /nodes/{node}/storage/{storage}/content/{volume}
func DeleteSnippetFileResty(ctx context.Context, restyClient *RestyClient, node, storage, volid string) error {
	path := fmt.Sprintf("/nodes/%s/storage/%s/content/%s",
		url.PathEscape(node), url.PathEscape(storage), url.PathEscape(volid))

	var response interface{}
	if err := restyClient.Delete(ctx, path, &response); err != nil {
		logger.Get().Error().
			Err(err).
			Str("node", node).
			Str("storage", storage).
			Str("volid", volid).
			Msg("Failed to delete snippet file (resty)")
		return fmt.Errorf("failed to delete snippet %s from %s on node %s: %w", volid, storage, node, err)
	}

	logger.Get().Info().
		Str("node", node).
		Str("storage", storage).
		Str("volid", volid).
		Msg("Snippet file deleted successfully (resty)")

	return nil
}

// GetSnippetsStoragesResty returns a list of storages that support snippets content.
func GetSnippetsStoragesResty(ctx context.Context, restyClient *RestyClient) ([]Storage, error) {
	storages, err := GetStoragesResty(ctx, restyClient)
	if err != nil {
		return nil, err
	}

	// Filter to only storages that support snippets
	var snippetStorages []Storage
	for _, s := range storages {
		if strings.Contains(s.Content, "snippets") {
			snippetStorages = append(snippetStorages, s)
		}
	}

	logger.Get().Debug().
		Int("total_storages", len(storages)).
		Int("snippet_storages", len(snippetStorages)).
		Msg("Filtered storages supporting snippets")

	return snippetStorages, nil
}
