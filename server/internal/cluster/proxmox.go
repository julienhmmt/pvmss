package cluster

import (
	"context"
)

// Proxmox is the real cluster implementation. It is a stub at T01 — it
// satisfies Client and Writer and returns ErrNotImplemented for every method
// except ConsoleRelay, which is wired in this tranche. BaseURL, APITokenName
// and APITokenValue are configured in main.go from the environment.
type Proxmox struct {
	BaseURL       string
	APITokenName  string
	APITokenValue string
}

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

// ListBridges implements Client.
func (Proxmox) ListBridges(_ context.Context) ([]Bridge, error) {
	return nil, ErrNotImplemented
}

// ListISOs implements Client.
func (Proxmox) ListISOs(_ context.Context) ([]ISOImage, error) {
	return nil, ErrNotImplemented
}

// ListPools implements Client.
func (Proxmox) ListPools(_ context.Context) ([]Pool, error) {
	return nil, ErrNotImplemented
}

// EnsurePoolRole implements Client.
func (Proxmox) EnsurePoolRole(_ context.Context) error {
	return ErrNotImplemented
}

// EnsurePoolUser implements Client.
func (Proxmox) EnsurePoolUser(_ context.Context, _, _ string) (string, error) {
	return "", ErrNotImplemented
}

// CreatePool implements Client.
func (Proxmox) CreatePool(_ context.Context, _, _ string) error {
	return ErrNotImplemented
}

// SetPoolACL implements Client.
func (Proxmox) SetPoolACL(_ context.Context, _, _, _ string) error {
	return ErrNotImplemented
}

// DeletePool implements Client.
func (Proxmox) DeletePool(_ context.Context, _ string) error {
	return ErrNotImplemented
}

// DeleteUser implements Client.
func (Proxmox) DeleteUser(_ context.Context, _ string) error {
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

// ListSnapshots implements SnapshotReader.
func (Proxmox) ListSnapshots(_ context.Context, _ string, _ int) ([]VMSnapshot, error) {
	return nil, ErrNotImplemented
}

// CreateSnapshot implements SnapshotWriter.
func (Proxmox) CreateSnapshot(_ context.Context, _ string, _ int, _, _ string, _ bool) (string, error) {
	return "", ErrNotImplemented
}

// RollbackSnapshot implements SnapshotWriter.
func (Proxmox) RollbackSnapshot(_ context.Context, _ string, _ int, _ string) (string, error) {
	return "", ErrNotImplemented
}

// DeleteSnapshot implements SnapshotWriter.
func (Proxmox) DeleteSnapshot(_ context.Context, _ string, _ int, _ string) (string, error) {
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
