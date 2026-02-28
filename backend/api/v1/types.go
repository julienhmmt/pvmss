package apiv1

// LoginRequest is the body for POST /api/v1/auth/login
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Admin    bool   `json:"admin"` // true → check ADMIN_PASSWORD_HASH
}

// AuthResponse is returned after a successful login or exchange
type AuthResponse struct {
	Username string `json:"username"`
	IsAdmin  bool   `json:"is_admin"`
}

// MeResponse is returned by GET /api/v1/auth/me
type MeResponse struct {
	Username string `json:"username"`
	IsAdmin  bool   `json:"is_admin"`
}

// VMSummary is the per-VM data returned in list and detail endpoints.
type VMSummary struct {
	VMID     int     `json:"vmid"`
	Name     string  `json:"name"`
	Node     string  `json:"node"`
	Status   string  `json:"status"`
	CPU      float64 `json:"cpu"` // fraction 0..1
	CPUs     int     `json:"cpus"`
	MemMB    int64   `json:"mem_mb"`     // current memory in MB
	MaxMemMB int64   `json:"max_mem_mb"` // allocated memory in MB
	DiskMB   int64   `json:"disk_mb"`
	Uptime   int64   `json:"uptime"` // seconds
	Tags     string  `json:"tags"`   // semicolon-separated
}

// VMListResponse wraps a slice of VMSummary
type VMListResponse struct {
	VMs   []VMSummary `json:"vms"`
	Total int         `json:"total"`
}

// VMActionRequest is the body for POST /api/v1/vms/:id/action
type VMActionRequest struct {
	Action string `json:"action"` // start|stop|shutdown|reboot|reset
	Node   string `json:"node"`
}

// VMActionResponse is returned after executing a VM action
type VMActionResponse struct {
	Success bool   `json:"success"`
	TaskID  string `json:"task_id,omitempty"` // Proxmox UPID when async
	Message string `json:"message,omitempty"`
}

// ErrorResponse is the standard JSON error body
type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// contextKey is an unexported type for context keys in this package
type contextKey string

const (
	contextKeyUsername contextKey = "username"
	contextKeyIsAdmin  contextKey = "is_admin"
)
