package main

// NodeLimit defines the resource limits (sockets, cores, RAM) for a specific Proxmox node.
// These are typically read-only and reflect the node's hardware capabilities.
type NodeLimit struct {
	Sockets MinMax `json:"sockets"`
	Cores   MinMax `json:"cores"`
	RAM     MinMax `json:"ram"`
}

// VMLimit defines the default resource limits for a new Virtual Machine.
// These values are used to populate the VM creation form.
type VMLimit struct {
	Sockets MinMax `json:"sockets"`
	Cores   MinMax `json:"cores"`
	RAM     MinMax `json:"ram"`
	Disk    MinMax `json:"disk"`
}

// IsDefined checks if the VMLimit has been populated with data.
func (v VMLimit) IsDefined() bool {
	// A simple check to see if the struct is likely to contain real data.
	// If Sockets.Min is set, we assume the rest of the data is intentional.
	return v.Sockets.Min > 0
}

// ResourceLimits defines a generic structure for resource limitations, including sockets, cores, RAM, and disk.
// The Disk field is optional to accommodate both node and VM limit types.
type ResourceLimits struct {
	Sockets MinMax  `json:"sockets"`
	Cores   MinMax  `json:"cores"`
	RAM     MinMax  `json:"ram"`
	Disk    *MinMax `json:"disk,omitempty"` // Only for VM limits
}
