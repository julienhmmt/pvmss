package proxmox

import (
	"context"
	"fmt"
	"net/url"

	"pvmss/logger"
)

// GetStoragesResty fetches the list of all storages from the `/storage` endpoint of the Proxmox API
// using resty for improved performance and error handling.
func GetStoragesResty(ctx context.Context, restyClient *RestyClient) ([]Storage, error) {
	var response ListResponse[Storage]

	if err := restyClient.Get(ctx, "/storage", &response); err != nil {
		logger.Get().Error().Err(err).Msg("Failed to fetch storages from Proxmox API (resty)")
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
			Msg("Parsed storage entry (resty)")
	}

	logger.Get().Info().Int("count", len(response.Data)).Msg("Successfully fetched storage list (resty)")
	return response.Data, nil
}

// GetNodeStoragesResty fetches storages for a specific node with status fields (used/total/avail) using resty.
// Endpoint: GET /nodes/{node}/storage
func GetNodeStoragesResty(ctx context.Context, restyClient *RestyClient, node string) ([]Storage, error) {
	var response ListResponse[Storage]
	path := "/nodes/" + url.PathEscape(node) + "/storage"

	if err := restyClient.Get(ctx, path, &response); err != nil {
		logger.Get().Error().Err(err).Str("node", node).Msg("Failed to fetch node storages from Proxmox API (resty)")
		return nil, fmt.Errorf("failed to fetch node storages: %w", err)
	}

	for i, storage := range response.Data {
		logger.Get().Debug().
			Int("index", i).
			Str("node", node).
			Str("storage", storage.Storage).
			Str("type", storage.Type).
			Str("used", storage.Used.String()).
			Str("total", storage.Total.String()).
			Str("avail", storage.Avail.String()).
			Int("active", storage.Active).
			Msg("Parsed node storage entry (resty)")
	}

	logger.Get().Info().Str("node", node).Int("count", len(response.Data)).Msg("Successfully fetched node storage list (resty)")
	return response.Data, nil
}
