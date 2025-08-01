package proxmox

import (
	"context"
	"encoding/json"
	"fmt"

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

// GetISOList retrieves the list of ISO images from a specific storage on a given Proxmox node.
// It uses the client's default timeout for the API request.
func GetISOList(client *Client, node string, storage string) ([]ISO, error) {
	logger.Get().Info().Str("node", node).Str("storage", storage).Msg("Fetching ISO list from Proxmox")
	ctx, cancel := context.WithTimeout(context.Background(), client.Timeout)
	defer cancel()
	return GetISOListWithContext(ctx, client, node, storage)
}

// GetISOListWithContext retrieves the list of ISO images from a specific storage on a given Proxmox node
// using the provided context for timeout and cancellation control.
// This is the underlying function that performs the actual API call.
func GetISOListWithContext(ctx context.Context, client *Client, node string, storage string) ([]ISO, error) {
	logger.Get().Info().Str("node", node).Str("storage", storage).Msg("Fetching ISO list with context from Proxmox")
	path := fmt.Sprintf("/nodes/%s/storage/%s/content", node, storage)
	data, err := client.GetWithContext(ctx, path)
	if err != nil {
		return nil, err
	}

	// The 'data' field from the Proxmox API response contains the list of ISOs.
	// We need to marshal it back to JSON and then unmarshal it into our typed slice.
	// This is a common pattern when dealing with nested, dynamic JSON in Go.
	var result struct {
		Data []ISO `json:"data"`
	}

	// Marshal the map[string]interface{} to a byte slice
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal iso list data: %w", err)
	}

	// Unmarshal the byte slice into our result struct
	if err := json.Unmarshal(jsonBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal iso list into typed struct: %w", err)
	}

	return result.Data, nil
}
