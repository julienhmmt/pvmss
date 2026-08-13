package httpapi

// HealthResponse is the public wire format for GET /health.
type HealthResponse struct {
	Status    string                 `json:"status"`
	Checks    map[string]CheckResult `json:"checks"`
	DemoMode  bool                   `json:"demoMode"`
	Timestamp string                 `json:"timestamp"`
}
