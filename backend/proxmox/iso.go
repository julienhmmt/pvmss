package proxmox

// ISO defines the structure of an ISO image as returned by the Proxmox API.
// We only map the fields that are relevant to the application.
// VolID: The unique volume identifier for the ISO (e.g., "storage-name:iso/filename.iso").
// Format: The file format, which is expected to be "iso".
// Size: The size of the ISO file in bytes.
type ISO struct {
	VolID  string `json:"volid"`
	Format string `json:"format"`
	Size   int64  `json:"size"`
}

// LEGACY FUNCTIONS REMOVED - Use resty versions instead:
// - GetISOList → GetISOListResty
// - GetISOListWithContext → GetISOListResty
// See resty_iso.go for modern implementations
