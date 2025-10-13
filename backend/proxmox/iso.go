package proxmox

import (
	"context"
	"fmt"
	"net/url"

	"pvmss/logger"
)

// ISO defines the structure of an ISO image as returned by the Proxmox API.
// We only map the fields that are relevant to the application.
// VolID: The unique volume identifier for the ISO (e.g., "storage-name:iso/filename.iso").
// Format: The file format, which is expected to be "iso".
// Size: The size of the ISO file in bytes.
type ISO struct {
	VolID  string `json:"volid"`
	Format string `json:"format"`
	Size   int64  `json:"size"`
}

// GetISOList fetches ISO files from a specific storage on a specific node in Proxmox node.
// It uses the client's default timeout for the API request.
func GetISOList(client ClientInterface, node, storage string) ([]ISO, error) {
	ctx, cancel := context.WithTimeout(context.Background(), client.GetTimeout())
	defer cancel()
	return GetISOListWithContext(ctx, client, node, storage)
}

// GetISOListWithContext retrieves the list of ISO images from a specific storage on a given Proxmox node
// using the provided context for timeout and cancellation control.
// This is the underlying function that performs the actual API call.
func GetISOListWithContext(ctx context.Context, client ClientInterface, node, storage string) ([]ISO, error) {
	if node == "" {
		return nil, fmt.Errorf("node name cannot be empty")
	}
	if storage == "" {
		return nil, fmt.Errorf("storage name cannot be empty")
	}

	path := fmt.Sprintf("/nodes/%s/storage/%s/content", url.PathEscape(node), url.PathEscape(storage))

	// Use the new GetJSON method to directly unmarshal into our typed response
	var response ListResponse[ISO]
	if err := client.GetJSON(ctx, path, &response); err != nil {
		logger.Get().Error().
			Err(err).
			Str("node", node).
			Str("storage", storage).
			Msg("Failed to get ISO list from Proxmox API")
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
		Int("count", len(isos)).
		Msg("Successfully fetched ISO list")

	return isos, nil
}
