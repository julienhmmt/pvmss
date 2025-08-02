package proxmox

import (
	"context"
	"encoding/json"
	"fmt"

	"pvmss/logger"
)

// Storage represents a Proxmox storage entry
type Storage struct {
	Storage     string      `json:"storage"`
	Type        string      `json:"type"`
	Used        json.Number `json:"used,omitempty"`
	Total       json.Number `json:"total,omitempty"`
	Avail       json.Number `json:"avail,omitempty"`
	Active      int         `json:"active,omitempty"`
	Enabled     int         `json:"enabled,omitempty"`
	Shared      int         `json:"shared,omitempty"`
	Content     string      `json:"content,omitempty"`
	Nodes       string      `json:"nodes,omitempty"`
	Description string      `json:"description,omitempty"`
}

// StorageListResponse represents the response from the /api2/json/storage endpoint
type StorageListResponse struct {
	Data []Storage `json:"data"`
}

// GetStorages is a convenience function that retrieves the list of all available storages across all nodes.
// It calls GetStoragesWithContext using the client's default timeout.
func GetStorages(client ClientInterface) ([]Storage, error) {
	logger.Get().Info().Msg("Fetching storage list from Proxmox")
	ctx, cancel := context.WithTimeout(context.Background(), client.GetTimeout())
	defer cancel()
	return GetStoragesWithContext(ctx, client)
}

// GetStoragesWithContext fetches the list of all storages from the `/storage` endpoint of the Proxmox API
// using the provided context for timeout and cancellation control.
func GetStoragesWithContext(ctx context.Context, client ClientInterface) ([]Storage, error) {
	// Use the new GetJSON method to directly unmarshal into our typed response
	var response ListResponse[Storage]
	if err := client.GetJSON(ctx, "/storage", &response); err != nil {
		logger.Get().Error().Err(err).Msg("Failed to fetch storages from Proxmox API")
		return nil, fmt.Errorf("failed to fetch storages: %w", err)
	}

	// Log parsed storages for debugging
	for i, storage := range response.Data {
		logger.Get().Debug().
			Int("index", i).
			Str("storage", storage.Storage).
			Str("type", storage.Type).
			Str("used", storage.Used.String()).
			Str("total", storage.Total.String()).
			Str("avail", storage.Avail.String()).
			Int("active", storage.Active).
			Msg("Parsed storage entry")
	}

	logger.Get().Info().Int("count", len(response.Data)).Msg("Successfully fetched storage list")
	return response.Data, nil
}
