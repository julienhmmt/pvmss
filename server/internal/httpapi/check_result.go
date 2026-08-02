package httpapi

// CheckResult is a single health check entry.
type CheckResult struct {
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}
