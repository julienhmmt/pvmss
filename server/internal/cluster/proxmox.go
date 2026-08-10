package cluster

import "context"

// Proxmox is the real cluster implementation. It is a stub at T01 — it
// satisfies Client and Writer and returns ErrNotImplemented. Filling it in
// belongs to the tranches that actually need reachable Proxmox data; building
// it now would be speculative work against a service nothing here can reach.
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

// GetCloudInitConfig implements CloudInitReader.
func (Proxmox) GetCloudInitConfig(_ context.Context, _ string, _ int) (CloudInitConfig, error) {
	return CloudInitConfig{}, ErrNotImplemented
}

// FindSnippetStorage implements CloudInitReader.
func (Proxmox) FindSnippetStorage(_ context.Context, _ string) (string, error) {
	return "", ErrNotImplemented
}

// Action implements Writer.
func (Proxmox) Action(_ context.Context, _ string, _ int, _ string) error {
	return ErrNotImplemented
}

// Delete implements Writer.
func (Proxmox) Delete(_ context.Context, _ string, _ int) error {
	return ErrNotImplemented
}

// Patch implements Writer.
func (Proxmox) Patch(_ context.Context, _ string, _ int, _, _ string) error {
	return ErrNotImplemented
}

// AddDisk implements Writer.
func (Proxmox) AddDisk(_ context.Context, _ string, _ int, _, _ string, _ int) (string, error) {
	return "", ErrNotImplemented
}

// ResizeDisk implements Writer.
func (Proxmox) ResizeDisk(_ context.Context, _ string, _ int, _ string, _ int) error {
	return ErrNotImplemented
}

// DeleteDisk implements Writer.
func (Proxmox) DeleteDisk(_ context.Context, _ string, _ int, _ string) error {
	return ErrNotImplemented
}

// SetCDROM implements Writer.
func (Proxmox) SetCDROM(_ context.Context, _ string, _ int, _ CDROMState) error {
	return ErrNotImplemented
}

// UpdateNetwork implements Writer.
func (Proxmox) UpdateNetwork(_ context.Context, _ string, _ int, _ []NetworkInterface) error {
	return ErrNotImplemented
}

// UpdateHardware implements Writer.
func (Proxmox) UpdateHardware(_ context.Context, _ string, _, _, _, _ int, _ []string) error {
	return ErrNotImplemented
}

// EnsureCloudInitDrive implements Writer.
func (Proxmox) EnsureCloudInitDrive(_ context.Context, _ string, _ int) error {
	return ErrNotImplemented
}

// SetCloudInitConfig implements Writer.
func (Proxmox) SetCloudInitConfig(_ context.Context, _ string, _ int, _ CloudInitConfig) error {
	return ErrNotImplemented
}

// PushCloudInitSnippet implements Writer.
func (Proxmox) PushCloudInitSnippet(_ context.Context, _, _, _ string, _ int, _ string) error {
	return ErrNotImplemented
}

// NextVMID implements Creator.
func (Proxmox) NextVMID(_ context.Context) (int, error) {
	return 0, ErrNotImplemented
}

// CreateVM implements Creator.
func (Proxmox) CreateVM(_ context.Context, _ VMSpec) (string, error) {
	return "", ErrNotImplemented
}

// TaskStatus implements Creator.
func (Proxmox) TaskStatus(_ context.Context, _ string) (TaskStatus, error) {
	return TaskStatus{}, ErrNotImplemented
}
