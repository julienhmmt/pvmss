package proxmox

import (
	"encoding/json"
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

// LEGACY FUNCTIONS REMOVED - Use resty versions instead:
// - GetStorages → GetStoragesResty
// - GetStoragesWithContext → GetStoragesResty
// - GetNodeStorages → GetNodeStoragesResty
// - GetNodeStoragesWithContext → GetNodeStoragesResty
// See resty_storage.go for modern implementations
