package apiv1

// --- Nodes ---

type AdminNodeResponse struct {
	Name      string  `json:"name"`
	Status    string  `json:"status"`
	CPU       float64 `json:"cpu"`
	MaxCPU    int     `json:"maxcpu"`
	Memory    float64 `json:"memory"`
	MaxMemory float64 `json:"max_memory"`
	Disk      float64 `json:"disk"`
	MaxDisk   float64 `json:"max_disk"`
	Uptime    int64   `json:"uptime"`
}

// --- Storage ---

type AdminStorageResponse struct {
	Storage string `json:"storage"`
	Type    string `json:"type"`
	Content string `json:"content"`
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

// --- Toggle Requests ---

type ToggleStorageRequest struct {
	Storage string `json:"storage"`
	Node    string `json:"node"`
}

type ToggleVMBRRequest struct {
	VMBR string `json:"vmbr"`
	Node string `json:"node"`
}

type ToggleISORequest struct {
	VolID string `json:"volid"`
}

// --- App Info ---

type AdminAppInfoResponse struct {
	Version          string                    `json:"version"`
	Environment      string                    `json:"environment"`
	GoVersion        string                    `json:"go_version"`
	Platform         string                    `json:"platform"`
	ProxmoxConnected bool                      `json:"proxmox_connected"`
	ProxmoxURL       string                    `json:"proxmox_url"`
	OfflineMode      bool                      `json:"offline_mode"`
	TotalNodes       int                       `json:"total_nodes"`
	TotalVMs         int                       `json:"total_vms"`
	ClusterInfo      *AdminClusterInfoResponse `json:"cluster_info,omitempty"`
	EnvVars          map[string]string         `json:"env_vars,omitempty"`
}

// AdminClusterInfoResponse represents Proxmox cluster information.
type AdminClusterInfoResponse struct {
	IsCluster   bool   `json:"is_cluster"`
	ClusterName string `json:"cluster_name"`
	NodeCount   int    `json:"node_count"`
}

// AdminSFTPStatusResponse represents SFTP configuration status for cloud-init uploads.
type AdminSFTPStatusResponse struct {
	Enabled    bool   `json:"enabled"`
	Host       string `json:"host,omitempty"`
	Username   string `json:"username,omitempty"`
	KeyExists  bool   `json:"key_exists"`
	StatusText string `json:"status_text"`
	StatusType string `json:"status_type"` // "success", "warning", "danger"
}

// AdminCloudInitListResponse wraps cloud-init templates with SFTP status.
type AdminCloudInitListResponse struct {
	Templates  []AdminCloudInitResponse `json:"templates"`
	SFTPStatus *AdminSFTPStatusResponse `json:"sftp_status,omitempty"`
}

// --- Admin Action ---

type AdminVMActionRequest struct {
	Action string `json:"action"`
}
