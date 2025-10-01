package proxmox

// Response is the base structure for all Proxmox API responses
// Used when the API returns a single object wrapped in {"data": {...}}
type Response[T any] struct {
	Data T `json:"data"`
}

// ListResponse is a generic response type for list endpoints
// Used when the API returns an array wrapped in {"data": [...]}
// This replaces all the specific *ListResponse types (VMListResponse, StorageListResponse, etc.)
type ListResponse[T any] struct {
	Data []T `json:"data"`
}
