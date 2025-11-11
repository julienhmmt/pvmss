// Package constants defines application-wide constants for timeouts, limits, and configuration values.
package constants

import "time"

// Context Timeouts
const (
	// DefaultContextTimeout is the default timeout for context operations
	DefaultContextTimeout = 10 * time.Second

	// LongContextTimeout is used for long-running operations
	LongContextTimeout = 30 * time.Second

	// ShortContextTimeout is used for quick operations
	ShortContextTimeout = 5 * time.Second

	// FetchVMsTimeout is the timeout for fetching VM lists
	FetchVMsTimeout = 15 * time.Second
)
