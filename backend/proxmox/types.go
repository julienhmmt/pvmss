package proxmox

// Response is the base structure for all Proxmox API responses
// Used when the API returns a single object wrapped in {"data": {...}}
type Response[T any] struct {
	Data T `json:"data"`
}
