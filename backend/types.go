package main

// MinMax defines a min/max value pair for resource limits
type MinMax struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

// AppSettings represents the application configuration
// This struct is used to unmarshal the settings.json file
// and make the configuration available throughout the application
type AppSettings struct {
	Tags          []string               `json:"tags"`
	ISOs          []string               `json:"isos"`
	VMBRs         []string               `json:"vmbrs"`
	Limits        map[string]interface{} `json:"limits"`
}

// VM represents a Proxmox Virtual Machine
type VM struct {
	ID     int    `json:"vmid"`
	Name   string `json:"name"`
	Status string `json:"status"`
	// Add other VM fields as needed
}

// StorageContent represents the content of a Proxmox storage
type StorageContent struct {
	VolID     string `json:"volid"`
	Size      int64  `json:"size"`
	Format    string `json:"format"`
	VMID      int    `json:"vmid,string"`
	CTID      int    `json:"ctid,string"`
	Content   string `json:"content"`
	Path      string `json:"path"`
	Used      int64  `json:"used"`
	Parent    string `json:"parent"`
	Notes     string `json:"notes"`
	Creation  string `json:"creation"`
	Verified  int    `json:"verified"`
	Encrypted int    `json:"encrypted"`
}

// Network represents a Proxmox network interface
type Network struct {
	Iface string `json:"iface"`
	// Add other network fields as needed
}
