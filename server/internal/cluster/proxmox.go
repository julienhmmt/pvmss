package cluster

import "context"

// Proxmox is the real cluster implementation. It is a stub at T01 — it
// satisfies Client and returns ErrNotImplemented. Filling it in belongs to
// the tranches that actually need reachable Proxmox data; building it now
// would be speculative work against a service nothing here can reach.
type Proxmox struct{}

// Snapshot implements Client.
func (Proxmox) Snapshot(_ context.Context) (Snapshot, error) {
	return Snapshot{}, ErrNotImplemented
}

// Authenticate implements Client.
func (Proxmox) Authenticate(_ context.Context, _, _ string) (Identity, error) {
	return Identity{}, ErrNotImplemented
}

// ChangePassword implements Client.
func (Proxmox) ChangePassword(_ context.Context, _, _, _ string) error {
	return ErrNotImplemented
}
