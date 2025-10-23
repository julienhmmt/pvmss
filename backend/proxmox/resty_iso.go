package proxmox

import (
	"context"
	"fmt"
	"net/url"

	"pvmss/logger"
)

// GetISOListResty retrieves the list of ISO images from a specific storage on a given Proxmox node
// using resty for improved performance and modern HTTP client features.
// Endpoint: GET /nodes/{node}/storage/{storage}/content
func GetISOListResty(ctx context.Context, restyClient *RestyClient, node, storage string) ([]ISO, error) {
	if node == "" {
		return nil, fmt.Errorf("node name cannot be empty")
	}
	if storage == "" {
		return nil, fmt.Errorf("storage name cannot be empty")
	}

	path := fmt.Sprintf("/nodes/%s/storage/%s/content", url.PathEscape(node), url.PathEscape(storage))

	var response ListResponse[ISO]
	if err := restyClient.Get(ctx, path, &response); err != nil {
		logger.Get().Error().
			Err(err).
			Str("node", node).
			Str("storage", storage).
			Msg("Failed to get ISO list from Proxmox API (resty)")
		return nil, fmt.Errorf("failed to get ISO list: %w", err)
	}

	// Filter for ISO files only
	var isos []ISO
	for _, item := range response.Data {
		if item.Format == "iso" {
			isos = append(isos, item)
		}
	}

	logger.Get().Info().
		Str("node", node).
		Str("storage", storage).
		Int("total_items", len(response.Data)).
		Int("iso_count", len(isos)).
		Msg("Successfully fetched ISO list (resty)")

	return isos, nil
}

// GetAllStorageContentResty retrieves all content (ISOs, disk images, etc.) from a storage
// This is useful if you want to see all types of files, not just ISOs
func GetAllStorageContentResty(ctx context.Context, restyClient *RestyClient, node, storage string) ([]ISO, error) {
	if node == "" {
		return nil, fmt.Errorf("node name cannot be empty")
	}
	if storage == "" {
		return nil, fmt.Errorf("storage name cannot be empty")
	}

	path := fmt.Sprintf("/nodes/%s/storage/%s/content", url.PathEscape(node), url.PathEscape(storage))

	var response ListResponse[ISO]
	if err := restyClient.Get(ctx, path, &response); err != nil {
		logger.Get().Error().
			Err(err).
			Str("node", node).
			Str("storage", storage).
			Msg("Failed to get storage content from Proxmox API (resty)")
		return nil, fmt.Errorf("failed to get storage content: %w", err)
	}

	logger.Get().Info().
		Str("node", node).
		Str("storage", storage).
		Int("total_items", len(response.Data)).
		Msg("Successfully fetched all storage content (resty)")

	return response.Data, nil
}
