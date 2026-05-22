package handlers

// JSONErrorResponse is the wire format for error responses returned by handlers
// outside the /api/v1/ stack (404/405 fallbacks, panic recovery, SPA bootstrap).
type JSONErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
