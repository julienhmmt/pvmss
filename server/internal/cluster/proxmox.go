package cluster

import "context"

// Proxmox is the real cluster implementation. It is a stub at T01 — it
// satisfies Client and returns ErrNotImplemented. Filling it in belongs to
// the tranches that actually need reachable Proxmox data; building it now
// would be speculative work against a service nothing here can reach.
type Proxmox struct{}

// ListNodes implements Client.
func (Proxmox) ListNodes(_ context.Context) ([]Node, error) {
	return nil, ErrNotImplemented
}
