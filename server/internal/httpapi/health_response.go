package httpapi

// HealthResponse is the public wire format for GET /health.
type HealthResponse struct {
	Status    string                 `json:"status"`
	Checks    map[string]CheckResult `json:"checks"`
	Timestamp string                 `json:"timestamp"`
}
