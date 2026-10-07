package httpapi

import "context"

// Pinger is the dependency the health check probes.
type Pinger interface {
	Ping(ctx context.Context) error
}
