// Package httpapi provides HTTP API handlers and types.
package httpapi

// CheckResult is a single health check entry.
type CheckResult struct {
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
	// Unreachable and Total carry the cluster counts as data so clients do
	// not have to parse them back out of the English Detail sentence.
	Unreachable int `json:"unreachable,omitempty"`
	Total       int `json:"total,omitempty"`
}
