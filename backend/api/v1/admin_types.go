package apiv1

// --- Nodes ---

type AdminNodeResponse struct {
	Name      string  `json:"name"`
	Status    string  `json:"status"`
	CPU       float64 `json:"cpu"`
	MaxCPU    int     `json:"maxcpu"`
	Memory    float64 `json:"memory"`
	MaxMemory float64 `json:"max_memory"`
	Uptime    int64   `json:"uptime"`
}

// --- Storage ---

type AdminStorageResponse struct {
	Storage string `json:"storage"`
	Type    string `json:"type"`
	Total   int64  `json:"total"`
	Used    int64  `json:"used"`
	Free    int64  `json:"free"`
	Node    string `json:"node"`
	Enabled bool   `json:"enabled"`
}

// --- VMs ---

type AdminVMResponse struct {
	VMID    int     `json:"vmid"`
	Name    string  `json:"name"`
	Node    string  `json:"node"`
	Status  string  `json:"status"`
	CPU     float64 `json:"cpu"`
	CPUs    int     `json:"cpus"`
	Mem     int64   `json:"mem"`
	MaxMem  int64   `json:"maxmem"`
	MaxDisk int64   `json:"maxdisk"`
	Uptime  int64   `json:"uptime"`
	Tags    string  `json:"tags"`
}

// --- User Pool ---

type AdminPoolResponse struct {
	PoolID  string   `json:"poolid"`
	Comment string   `json:"comment"`
	Members []string `json:"members"`
}

type CreatePoolRequest struct {
	Pool     string `json:"pool"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// --- Tags ---

type AdminTagResponse struct {
	Name    string `json:"name"`
	VMCount int    `json:"vm_count"`
}

type CreateTagRequest struct {
	Name string `json:"name"`
}

// --- Limits ---

type AdminLimitsResponse struct {
	VM              VMResourceLimitsResponse              `json:"vm"`
	Nodes           map[string]NodeResourceLimitsResponse `json:"nodes"`
	MaxSnapshots    int                                   `json:"max_snapshots"`
	MaxNetworkCards int                                   `json:"max_network_cards"`
	MaxDiskPerVM    int                                   `json:"max_disk_per_vm"`
	MaxVMPerUser    int                                   `json:"max_vm_per_user"`
}

type ResourceRangeResponse struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

type VMResourceLimitsResponse struct {
	Sockets ResourceRangeResponse `json:"sockets"`
	Cores   ResourceRangeResponse `json:"cores"`
	RAM     ResourceRangeResponse `json:"ram"`
	Disk    ResourceRangeResponse `json:"disk"`
}

type NodeResourceLimitsResponse struct {
	Sockets ResourceRangeResponse `json:"sockets"`
	Cores   ResourceRangeResponse `json:"cores"`
	RAM     ResourceRangeResponse `json:"ram"`
	Disk    ResourceRangeResponse `json:"disk"`
}

// --- VMBR ---

type AdminVMBRResponse struct {
	Iface       string `json:"iface"`
	Type        string `json:"type"`
	Active      bool   `json:"active"`
	BridgePorts string `json:"bridge_ports"`
	Node        string `json:"node"`
	Enabled     bool   `json:"enabled"`
}

// --- Cloud-Init ---

type AdminCloudInitResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Storage     string `json:"storage"`
	Filename    string `json:"filename"`
	Enabled     bool   `json:"enabled"`
}

type CreateCloudInitRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Storage     string `json:"storage"`
	YAMLContent string `json:"yaml_content"`
}

type UpdateCloudInitRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Storage     string `json:"storage"`
	YAMLContent string `json:"yaml_content"`
}

// --- ISO ---

type AdminISOResponse struct {
	VolID   string `json:"volid"`
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	Storage string `json:"storage"`
	Enabled bool   `json:"enabled"`
}

// --- App Info ---

type AdminAppInfoResponse struct {
	Version          string `json:"version"`
	Environment      string `json:"environment"`
	ProxmoxConnected bool   `json:"proxmox_connected"`
	ProxmoxURL       string `json:"proxmox_url"`
	OfflineMode      bool   `json:"offline_mode"`
	TotalNodes       int    `json:"total_nodes"`
	TotalVMs         int    `json:"total_vms"`
}

// --- Admin Action ---

type AdminVMActionRequest struct {
	Action string `json:"action"`
}
