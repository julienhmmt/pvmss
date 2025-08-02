package proxmox

// Response is the base structure for all Proxmox API responses
type Response[T any] struct {
	Data T `json:"data"`
}

// ListResponse is a generic response type for list endpoints
type ListResponse[T any] struct {
	Data []T `json:"data"`
}

// ErrorResponse represents an error response from the Proxmox API
type ErrorResponse struct {
	Message string `json:"message,omitempty"`
	Errors  any    `json:"errors,omitempty"`
}

// Error implements the error interface
func (e *ErrorResponse) Error() string {
	return e.Message
}
