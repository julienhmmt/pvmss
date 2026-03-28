# Admin SvelteKit SPA — Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the Go templ admin pages with a SvelteKit SPA served at `/admin/*`, including a full JSON admin API, while keeping user-facing pages untouched.

**Architecture:** SvelteKit 2 (adapter-static, `base: '/admin'`) builds to `frontend-svelte/build/`. Go serves the build as static files at `/admin/*` via the top-level `http.ServeMux`. New `/api/v1/admin/*` endpoints use the existing `JWTAdminMiddleware` (cookie-based JWT, `is_admin` claim). Auth uses the existing `exchange` endpoint — no JWT refactor.

**Tech Stack:** SvelteKit 2, Svelte 5, TypeScript, Vite 5, shadcn-svelte (Mira preset), Tailwind CSS v4, Geist font, Phosphor icons, Go 1.25, httprouter, go-resty

**Design spec:** `docs/superpowers/specs/2026-03-15-admin-svelte-design.md`

---

## File Map

### New Go files

| File | Responsibility |
|------|---------------|
| `backend/api/v1/admin_handlers.go` | Read-only admin API handlers: nodes, storage, vmbr, iso, appinfo, settings |
| `backend/api/v1/admin_mutations.go` | Write admin API handlers: userpool CRUD, tags CRUD, limits PUT, cloudinit CRUD+toggle |
| `backend/api/v1/admin_vms.go` | Admin all-VMs list + per-VM action handler |
| `backend/api/v1/admin_types.go` | JSON response types for admin endpoints |
| `backend/api/v1/admin_handlers_test.go` | Tests for read-only admin handlers |
| `backend/api/v1/admin_mutations_test.go` | Tests for mutation admin handlers |
| `backend/api/v1/admin_vms_test.go` | Tests for admin VMs handler |

### Modified Go files

| File | Change |
|------|--------|
| `backend/api/v1/router.go` | Register all `/api/v1/admin/*` routes with `adminJWTWrap` |
| `backend/handlers/handlers.go` | Add `/admin/assets/` file server + `/admin/` SPA fallback to mux |

### New frontend-svelte/ (SvelteKit project)

```
frontend-svelte/
  package.json, tsconfig.json, svelte.config.js, vite.config.ts
  tailwind.config.ts, components.json, app.html, app.css
  src/
    lib/
      api/client.ts
      api/auth.ts
      api/admin/nodes.ts, storage.ts, vms.ts, userpool.ts,
                   tags.ts, limits.ts, vmbr.ts, cloudinit.ts,
                   iso.ts, appinfo.ts
      components/ui/          ← shadcn-svelte generated
      components/layout/AppShell.svelte, AdminSidebar.svelte,
                        PageHeader.svelte, ThemeToggle.svelte
      components/data/DataTable.svelte, ResourceCard.svelte,
                      StatusBadge.svelte, EmptyState.svelte,
                      LoadingSkeleton.svelte
      components/forms/ConfirmDialog.svelte, InlineEdit.svelte, TagInput.svelte
      components/feedback/ErrorBanner.svelte
      stores/auth.svelte.ts, theme.svelte.ts
      types/admin.ts, api.ts
      utils/format.ts
    routes/
      +layout.svelte, +layout.ts, +page.svelte
      admin/+layout.svelte, +page.svelte
      admin/nodes/+page.svelte, storage/+page.svelte, vms/+page.svelte,
            userpool/+page.svelte, tags/+page.svelte, limits/+page.svelte,
            vmbr/+page.svelte, cloudinit/+page.svelte, iso/+page.svelte,
            appinfo/+page.svelte
```

---

## Chunk 1: Backend Admin API

### Task 1: Admin response types

**Files:**
- Create: `backend/api/v1/admin_types.go`

- [ ] **Step 1: Create admin response types**

```go
// backend/api/v1/admin_types.go
package apiv1

import "pvmss/state"

// --- Nodes ---
// Maps from proxmox.NodeDetails: Node(string), Status(string), CPU(float64),
// MaxCPU(int), Memory(float64), MaxMemory(float64), Uptime(int64)

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
// Maps from proxmox.Storage: fields Total/Used/Avail are json.Number — use .Int64()

type AdminStorageResponse struct {
	Storage string `json:"storage"`
	Type    string `json:"type"`
	Total   int64  `json:"total"`
	Used    int64  `json:"used"`
	Free    int64  `json:"free"`
	Node    string `json:"node"`
	Enabled bool   `json:"enabled"`
}
// Mapper: s.Total.Int64(), s.Used.Int64(), s.Avail.Int64() for conversion

// --- VMs ---
// Maps from proxmox.VM: VMID(int), Name, Node, Status, CPU(float64), CPUs(int) [NOT MaxCPU],
// Mem(int64), MaxMem(int64), MaxDisk(int64), Uptime(int64), Tags(string)
// NOTE: proxmox.VM has NO Pool or Disk field. Pool must come from config or be omitted.

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
	VM             state.VMResourceLimits              `json:"vm"`
	Nodes          map[string]state.NodeResourceLimits  `json:"nodes"`
	MaxSnapshots   int                                  `json:"max_snapshots"`
	MaxNetworkCards int                                 `json:"max_network_cards"`
	MaxDiskPerVM   int                                  `json:"max_disk_per_vm"`
	MaxVMPerUser   int                                  `json:"max_vm_per_user"`
}

// --- VMBR ---
// Maps from proxmox.VMBR: Iface(string), Type(string), Active(any), BridgePorts(string)
// "Enabled" comes from settings (h.state.GetVMBRs()), not from Proxmox

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
	LogLevel         string `json:"log_level"`
	LogFormat        string `json:"log_format"`
	TotalNodes       int    `json:"total_nodes"`
	TotalVMs         int    `json:"total_vms"`
}

// --- Admin Action ---

type AdminVMActionRequest struct {
	Action string `json:"action"`
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd backend && go build ./...`
Expected: PASS (no errors)

- [ ] **Step 3: Commit**

```bash
git add backend/api/v1/admin_types.go
git commit -m "feat(api): add admin API response types"
```

---

### Task 2: Read-only admin handlers (nodes, storage, vmbr, iso, appinfo)

**Files:**
- Create: `backend/api/v1/admin_handlers.go`
- Create: `backend/api/v1/admin_handlers_test.go`

- [ ] **Step 1: Write tests for read-only handlers**

Test in offline mode (no Proxmox connection). Each handler should return 200 with valid JSON. Test that non-admin JWT returns 403.

Key test cases:
- `GET /api/v1/admin/nodes` → 200 with `[]AdminNodeResponse` (empty in offline mode)
- `GET /api/v1/admin/storage` → 200 with `[]AdminStorageResponse`
- `GET /api/v1/admin/vmbr` → 200 with `[]AdminVMBRResponse`
- `GET /api/v1/admin/iso` → 200 with `[]AdminISOResponse`
- `GET /api/v1/admin/appinfo` → 200 with `AdminAppInfoResponse`
- `GET /api/v1/admin/settings` → 200 with settings JSON
- All endpoints → 403 with non-admin JWT cookie
- All endpoints → 401 with no JWT cookie

Reference the existing test patterns in `backend/api/v1/auth_test.go` and `backend/api/v1/vms_test.go` for how to set up test JWT cookies and the test app.

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd backend && go test ./api/v1/ -run TestAdmin -v`
Expected: FAIL (handlers not implemented)

- [ ] **Step 3: Implement read-only handlers**

```go
// backend/api/v1/admin_handlers.go
package apiv1

import (
	"net/http"
	"os"
	"time"

	"pvmss/proxmox"
	"pvmss/state"
)

type AdminHandler struct {
	state state.StateManager
}

func MakeAdminHandler(s state.StateManager) *AdminHandler {
	return &AdminHandler{state: s}
}

func (h *AdminHandler) Nodes(w http.ResponseWriter, r *http.Request) {
	if h.state.IsOfflineMode() {
		writeJSON(w, []AdminNodeResponse{})
		return
	}
	restyClient, err := proxmox.MakeRestyClientFromEnv(10 * time.Second)
	if err != nil {
		errInternal(w)
		return
	}
	names, err := proxmox.GetNodeNamesResty(r.Context(), restyClient)
	if err != nil {
		errInternal(w)
		return
	}
	// Use cached node details from state if available
	cached, _ := h.state.GetNodeCache()
	result := make([]AdminNodeResponse, 0, len(names))
	for _, details := range cached {
		result = append(result, AdminNodeResponse{
			Name:      details.Node,   // NodeDetails.Node, not .Name
			Status:    details.Status,
			CPU:       details.CPU,
			MaxCPU:    details.MaxCPU,
			Memory:    details.Memory,    // float64, not int64
			MaxMemory: details.MaxMemory, // float64, not int64
			Uptime:    details.Uptime,
		})
	}
	// Fallback: if cache is empty, return names only
	if len(result) == 0 {
		for _, name := range names {
			result = append(result, AdminNodeResponse{Name: name, Status: "unknown"})
		}
	}
	writeJSON(w, result)
}

func (h *AdminHandler) Storage(w http.ResponseWriter, r *http.Request) {
	if h.state.IsOfflineMode() {
		writeJSON(w, []AdminStorageResponse{})
		return
	}
	restyClient, err := proxmox.MakeRestyClientFromEnv(10 * time.Second)
	if err != nil {
		errInternal(w)
		return
	}
	storages, err := proxmox.GetStoragesResty(r.Context(), restyClient)
	if err != nil {
		errInternal(w)
		return
	}
	enabled := h.state.GetStorages()
	enabledSet := make(map[string]bool, len(enabled))
	for _, s := range enabled {
		enabledSet[s] = true
	}
	result := make([]AdminStorageResponse, 0, len(storages))
	for _, s := range storages {
		total, _ := s.Total.Int64()    // json.Number → int64
		used, _ := s.Used.Int64()
		free, _ := s.Avail.Int64()
		result = append(result, AdminStorageResponse{
			Storage: s.Storage,
			Type:    s.Type,
			Total:   total,
			Used:    used,
			Free:    free,
			Node:    s.Nodes,  // Storage.Nodes is a string (comma-separated node names)
			Enabled: enabledSet[s.Storage],
		})
	}
	writeJSON(w, result)
}
```

Continue with VMBR, ISO, AppInfo, Settings handlers following the same pattern. Each reads from Proxmox via resty and/or from `h.state.GetSettings()` and maps to the admin response types.

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd backend && go test ./api/v1/ -run TestAdmin -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/api/v1/admin_handlers.go backend/api/v1/admin_handlers_test.go
git commit -m "feat(api): add read-only admin handlers (nodes, storage, vmbr, iso, appinfo)"
```

---

### Task 3: Admin VMs handler

**Files:**
- Create: `backend/api/v1/admin_vms.go`
- Create: `backend/api/v1/admin_vms_test.go`

- [ ] **Step 1: Write tests**

Key test cases:
- `GET /api/v1/admin/vms` → 200 with `[]AdminVMResponse`
- `POST /api/v1/admin/vms/100/action` with `{"action":"stop"}` → 200 with `VMActionResponse`
- `POST /api/v1/admin/vms/100/action` with `{"action":"invalid"}` → 400
- All → 403 without admin

- [ ] **Step 2: Run tests — expect FAIL**

Run: `cd backend && go test ./api/v1/ -run TestAdminVM -v`

- [ ] **Step 3: Implement admin VMs handler**

```go
// backend/api/v1/admin_vms.go
package apiv1

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/julienschmidt/httprouter"
	"pvmss/proxmox"
	"pvmss/state"
)

type AdminVMsAPIHandler struct {
	state state.StateManager
}

func MakeAdminVMsAPIHandler(s state.StateManager) *AdminVMsAPIHandler {
	return &AdminVMsAPIHandler{state: s}
}

func (h *AdminVMsAPIHandler) ListAllVMs(w http.ResponseWriter, r *http.Request) {
	if h.state.IsOfflineMode() {
		writeJSON(w, []AdminVMResponse{})
		return
	}
	restyClient, err := proxmox.MakeRestyClientFromEnv(10 * time.Second)
	if err != nil {
		errInternal(w)
		return
	}
	vms, err := proxmox.GetVMsResty(r.Context(), restyClient)
	if err != nil {
		errInternal(w)
		return
	}
	result := make([]AdminVMResponse, 0, len(vms))
	for _, vm := range vms {
		result = append(result, AdminVMResponse{
			VMID:    vm.VMID,
			Name:    vm.Name,
			Node:    vm.Node,
			Status:  vm.Status,
			CPU:     vm.CPU,
			CPUs:    vm.CPUs,     // proxmox.VM uses CPUs, not MaxCPU
			Mem:     vm.Mem,
			MaxMem:  vm.MaxMem,
			MaxDisk: vm.MaxDisk,  // No Disk field in proxmox.VM, only MaxDisk
			Uptime:  vm.Uptime,
			Tags:    vm.Tags,
		})
	}
	writeJSON(w, result)
}

func (h *AdminVMsAPIHandler) VMAction(w http.ResponseWriter, r *http.Request) {
	if h.state.IsOfflineMode() {
		errOffline(w)
		return
	}
	ps := r.Context().Value(httprouter.ParamsKey).(httprouter.Params)
	vmid := ps.ByName("id")     // vmid is string — VMActionResty takes string
	if vmid == "" {
		errBadRequest(w, "missing VM ID")
		return
	}
	var req AdminVMActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errBadRequest(w, "invalid JSON body")
		return
	}
	validActions := map[string]bool{"start": true, "stop": true, "shutdown": true, "reboot": true}
	if !validActions[req.Action] {
		errBadRequest(w, "invalid action: must be start, stop, shutdown, or reboot")
		return
	}
	restyClient, err := proxmox.MakeRestyClientFromEnv(10 * time.Second)
	if err != nil {
		errInternal(w)
		return
	}
	// Find VM node
	vms, err := proxmox.GetVMsResty(r.Context(), restyClient)
	if err != nil {
		errInternal(w)
		return
	}
	vmidInt, _ := strconv.Atoi(vmid) // for comparison with proxmox.VM.VMID (int)
	var node string
	for _, vm := range vms {
		if vm.VMID == vmidInt {
			node = vm.Node
			break
		}
	}
	if node == "" {
		errBadRequest(w, "VM not found")
		return
	}
	taskID, err := proxmox.VMActionResty(r.Context(), restyClient, node, vmid, req.Action) // vmid is string
	if err != nil {
		writeError(w, http.StatusInternalServerError, "vm_action_failed", err.Error())
		return
	}
	writeJSON(w, VMActionResponse{Success: true, TaskID: taskID})
}
```

- [ ] **Step 4: Run tests — expect PASS**

Run: `cd backend && go test ./api/v1/ -run TestAdminVM -v`

- [ ] **Step 5: Commit**

```bash
git add backend/api/v1/admin_vms.go backend/api/v1/admin_vms_test.go
git commit -m "feat(api): add admin VMs handler (list all + action)"
```

---

### Task 4: Admin mutation handlers (userpool, tags, limits, cloudinit)

**Files:**
- Create: `backend/api/v1/admin_mutations.go`
- Create: `backend/api/v1/admin_mutations_test.go`

- [ ] **Step 1: Write tests for userpool CRUD**

Key test cases:
- `GET /api/v1/admin/userpool` → 200 with `[]AdminPoolResponse`
- `POST /api/v1/admin/userpool` with `{"pool":"test","username":"user","password":"pass"}` → 201
- `DELETE /api/v1/admin/userpool/test` → 204
- `POST` with missing fields → 400

- [ ] **Step 2: Write tests for tags CRUD**

- `GET /api/v1/admin/tags` → 200 with `[]AdminTagResponse`
- `POST /api/v1/admin/tags` with `{"name":"newtag"}` → 201
- `DELETE /api/v1/admin/tags/newtag` → 204
- `POST` with empty name → 400

- [ ] **Step 3: Write tests for limits PUT**

- `GET /api/v1/admin/limits` → 200 with `AdminLimitsResponse`
- `PUT /api/v1/admin/limits` with valid limits JSON → 204

- [ ] **Step 4: Write tests for cloudinit CRUD**

- `GET /api/v1/admin/cloudinit` → 200 with `[]AdminCloudInitResponse`
- `POST /api/v1/admin/cloudinit` → 201
- `PUT /api/v1/admin/cloudinit/:id` → 204
- `DELETE /api/v1/admin/cloudinit/:id` → 204
- `POST /api/v1/admin/cloudinit/:id/toggle` → 204

- [ ] **Step 5: Run all tests — expect FAIL**

Run: `cd backend && go test ./api/v1/ -run TestAdminMutation -v`

- [ ] **Step 6: Implement mutation handlers**

Implementation approach for each:
- **UserPool**: Use existing `proxmox.EnsureUserResty`, `proxmox.EnsurePoolResty`, `proxmox.EnsurePoolACLResty`, `proxmox.EnsureRoleResty` — same logic as `backend/handlers/user_pool.go`
- **Tags**: Read/write `h.state.GetSettings().Tags` via `h.state.SetSettings()` — same as `backend/handlers/tags.go`, plus count VMs per tag using `proxmox.GetVMsResty`
- **Limits**: Read/write `h.state.GetSettings().Limits`, `.MaxNetworkCards`, `.MaxDiskPerVM`, `.MaxVMPerUser` — same as `backend/handlers/settings_limits.go`
- **CloudInit**: Read/write `h.state.GetSettings().CloudInitTemplates` — same as `backend/handlers/admin_cloudinit.go`

All mutations follow the pattern:
```go
settings := h.state.GetSettings()
// Create new copy for immutability — deep copy maps to avoid shared references
newSettings := *settings
// For nested maps (e.g., Limits.Nodes), rebuild the map:
newSettings.Limits.Nodes = make(map[string]state.NodeResourceLimits, len(settings.Limits.Nodes))
for k, v := range settings.Limits.Nodes {
	newSettings.Limits.Nodes[k] = v
}
// Apply change to newSettings...
if err := h.state.SetSettings(&newSettings); err != nil {
	errInternal(w)
	return
}
```

- [ ] **Step 7: Run all tests — expect PASS**

Run: `cd backend && go test ./api/v1/ -run TestAdminMutation -v`

- [ ] **Step 8: Commit**

```bash
git add backend/api/v1/admin_mutations.go backend/api/v1/admin_mutations_test.go
git commit -m "feat(api): add admin mutation handlers (userpool, tags, limits, cloudinit)"
```

---

### Task 5: Register admin routes + SPA serving

**Files:**
- Modify: `backend/api/v1/router.go`
- Modify: `backend/handlers/handlers.go`

- [ ] **Step 1: Write test for route registration**

Add to existing route tests: verify all admin routes return 200 with admin JWT and 401/403 without.

- [ ] **Step 2: Run test — expect FAIL**

- [ ] **Step 3: Register admin API routes in router.go**

```go
// Add to RegisterRoutes in backend/api/v1/router.go
func RegisterRoutes(router *httprouter.Router, s state.StateManager) {
	// ... existing routes ...

	// Admin API routes — JWT + isAdmin required
	adminHandler := MakeAdminHandler(s)
	adminVMsHandler := MakeAdminVMsAPIHandler(s)
	adminMutHandler := MakeAdminMutationsHandler(s)

	router.GET("/api/v1/admin/nodes", adminJWTWrap(s, adminHandler.Nodes))
	router.GET("/api/v1/admin/storage", adminJWTWrap(s, adminHandler.Storage))
	router.GET("/api/v1/admin/vmbr", adminJWTWrap(s, adminHandler.VMBR))
	router.GET("/api/v1/admin/iso", adminJWTWrap(s, adminHandler.ISO))
	router.GET("/api/v1/admin/appinfo", adminJWTWrap(s, adminHandler.AppInfo))
	router.GET("/api/v1/admin/settings", adminJWTWrap(s, adminHandler.Settings))

	router.GET("/api/v1/admin/vms", adminJWTWrap(s, adminVMsHandler.ListAllVMs))
	router.POST("/api/v1/admin/vms/:id/action", adminJWTWrap(s, adminVMsHandler.VMAction))

	router.GET("/api/v1/admin/userpool", adminJWTWrap(s, adminMutHandler.ListPools))
	router.POST("/api/v1/admin/userpool", adminJWTWrap(s, adminMutHandler.CreatePool))
	router.DELETE("/api/v1/admin/userpool/:name", adminJWTWrap(s, adminMutHandler.DeletePool))

	router.GET("/api/v1/admin/tags", adminJWTWrap(s, adminMutHandler.ListTags))
	router.POST("/api/v1/admin/tags", adminJWTWrap(s, adminMutHandler.CreateTag))
	router.DELETE("/api/v1/admin/tags/:name", adminJWTWrap(s, adminMutHandler.DeleteTag))

	router.GET("/api/v1/admin/limits", adminJWTWrap(s, adminMutHandler.GetLimits))
	router.PUT("/api/v1/admin/limits", adminJWTWrap(s, adminMutHandler.UpdateLimits))

	router.GET("/api/v1/admin/cloudinit", adminJWTWrap(s, adminMutHandler.ListCloudInit))
	router.POST("/api/v1/admin/cloudinit", adminJWTWrap(s, adminMutHandler.CreateCloudInit))
	router.PUT("/api/v1/admin/cloudinit/:id", adminJWTWrap(s, adminMutHandler.UpdateCloudInit))
	router.DELETE("/api/v1/admin/cloudinit/:id", adminJWTWrap(s, adminMutHandler.DeleteCloudInit))
	router.POST("/api/v1/admin/cloudinit/:id/toggle", adminJWTWrap(s, adminMutHandler.ToggleCloudInit))
}

// adminJWTWrap wraps a handler with JWT + isAdmin check.
func adminJWTWrap(s state.StateManager, h http.HandlerFunc) httprouter.Handle {
	return httprouterWrap(JWTAdminMiddleware(s, h))
}
```

- [ ] **Step 4: Add SPA serving to handlers.go mux**

In `handlers.go`, add before the existing `mux.HandleFunc("/", ...)`:

```go
// Serve SvelteKit admin SPA static assets
spaDir := filepath.Join(stateManager.GetFrontendPath(), "..", "frontend-svelte", "build")
mux.Handle("/admin/assets/", http.StripPrefix("/admin/assets/", http.FileServer(http.Dir(filepath.Join(spaDir, "assets")))))

// SPA fallback for all /admin/* routes
spaIndex := filepath.Join(spaDir, "index.html")
mux.HandleFunc("/admin/", func(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, spaIndex)
})
```

- [ ] **Step 5: Run all tests**

Run: `cd backend && go test ./... -tags=offline -count=1 -v 2>&1 | tail -20`
Expected: PASS (no regressions on existing tests)

- [ ] **Step 6: Commit**

```bash
git add backend/api/v1/router.go backend/handlers/handlers.go
git commit -m "feat(api): register admin routes and add SPA serving"
```

---

## Chunk 2: SvelteKit Scaffold & Component Library

### Task 6: Scaffold SvelteKit project

**Files:**
- Create: `frontend-svelte/` (entire SvelteKit project)

- [ ] **Step 1: Create SvelteKit project**

```bash
cd /Users/jh/git/gh/pvmss
npm create svelte@latest frontend-svelte -- --template skeleton --types typescript
cd frontend-svelte
npm install
```

- [ ] **Step 2: Install adapter-static**

```bash
npm i -D @sveltejs/adapter-static
```

- [ ] **Step 3: Configure svelte.config.js**

```javascript
import adapter from '@sveltejs/adapter-static';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

/** @type {import('@sveltejs/kit').Config} */
const config = {
	preprocess: vitePreprocess(),
	kit: {
		adapter: adapter({
			fallback: 'index.html'
		}),
		paths: {
			base: '/admin'
		}
	}
};

export default config;
```

- [ ] **Step 4: Configure vite.config.ts**

```typescript
import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

export default defineConfig({
	plugins: [sveltekit()],
	server: {
		proxy: {
			'/api': {
				target: 'http://localhost:50000',
				changeOrigin: true
			}
		}
	}
});
```

- [ ] **Step 5: Verify the scaffold builds**

```bash
cd frontend-svelte && npm run build
```
Expected: Build succeeds, `build/` directory created with `index.html`

- [ ] **Step 6: Commit**

```bash
git add frontend-svelte/
git commit -m "feat(frontend): scaffold SvelteKit project with adapter-static"
```

---

### Task 7: Set up shadcn-svelte + Mira preset

**Files:**
- Modify: `frontend-svelte/` (shadcn init + components)

- [ ] **Step 1: Initialize shadcn-svelte**

```bash
cd frontend-svelte
npx shadcn-svelte@latest init --preset a1DMDThI
```

If the preset flag doesn't work, use the interactive init and select: style=mira, baseColor=stone, theme=orange, font=geist, icons=phosphor, radius=small.

- [ ] **Step 2: Install Geist font + Phosphor icons**

```bash
npm i @fontsource/geist-sans @fontsource/geist-mono phosphor-svelte
```

- [ ] **Step 3: Add shadcn components**

```bash
npx shadcn-svelte@latest add button card dialog table form input select tabs badge dropdown-menu sidebar sheet sonner skeleton separator tooltip switch
```

- [ ] **Step 4: Verify build**

```bash
npm run build
```
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add -A frontend-svelte/
git commit -m "feat(frontend): add shadcn-svelte Mira preset with core UI components"
```

---

### Task 8: TypeScript types + API client

**Files:**
- Create: `frontend-svelte/src/lib/types/api.ts`
- Create: `frontend-svelte/src/lib/types/admin.ts`
- Create: `frontend-svelte/src/lib/api/client.ts`
- Create: `frontend-svelte/src/lib/api/auth.ts`
- Create: `frontend-svelte/src/lib/utils/format.ts`

- [ ] **Step 1: Create API types**

```typescript
// src/lib/types/api.ts
export interface ApiError {
  code: string
  message: string
}

export class ApiRequestError extends Error {
  constructor(
    public readonly status: number,
    public readonly error: ApiError
  ) {
    super(error.message)
  }
}
```

- [ ] **Step 2: Create admin types**

```typescript
// src/lib/types/admin.ts
// Mirror the Go admin_types.go exactly
export interface Node {
  name: string
  status: string
  cpu: number
  maxcpu: number
  mem: number
  maxmem: number
  uptime: number
}

export interface Storage {
  storage: string
  type: string
  total: number
  used: number
  free: number
  node: string
  enabled: boolean
}

export interface VM {
  vmid: number
  name: string
  node: string
  status: string
  pool: string
  cpu: number
  maxcpu: number
  mem: number
  maxmem: number
  disk: number
  maxdisk: number
  uptime: number
  tags: string
}

export interface Pool {
  poolid: string
  comment: string
  members: string[]
}

export interface Tag {
  name: string
  vm_count: number
}

export interface ResourceRange {
  min: number
  max: number
}

export interface Limits {
  vm: {
    sockets: ResourceRange
    cores: ResourceRange
    ram: ResourceRange
    disk: ResourceRange
  }
  nodes: Record<string, {
    sockets: ResourceRange
    cores: ResourceRange
    ram: ResourceRange
    disk: ResourceRange
  }>
  max_snapshots: number
  max_network_cards: number
  max_disk_per_vm: number
  max_vm_per_user: number
}

export interface VMBR {
  name: string
  type: string
  active: boolean
  node: string
  enabled: boolean
}

export interface CloudInitTemplate {
  id: string
  name: string
  description: string
  storage: string
  filename: string
  enabled: boolean
}

export interface ISO {
  volid: string
  name: string
  size: number
  storage: string
  enabled: boolean
}

export interface AppInfo {
  version: string
  environment: string
  proxmox_connected: boolean
  proxmox_url: string
  offline_mode: boolean
  log_level: string
  log_format: string
  total_nodes: number
  total_vms: number
}

export type VMAction = 'start' | 'stop' | 'shutdown' | 'reboot'
```

- [ ] **Step 3: Create API client**

```typescript
// src/lib/api/client.ts
import { ApiRequestError, type ApiError } from '$lib/types/api'

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const res = await fetch(path, {
    credentials: 'same-origin', // send cookies (access_token, refresh_token)
    headers: {
      'Content-Type': 'application/json',
      ...options.headers
    },
    ...options
  })

  if (res.status === 401) {
    // Try refresh
    const refreshRes = await fetch('/api/v1/auth/refresh', {
      method: 'POST',
      credentials: 'same-origin'
    })
    if (refreshRes.ok) {
      // Retry original request
      const retryRes = await fetch(path, {
        credentials: 'same-origin',
        headers: { 'Content-Type': 'application/json', ...options.headers },
        ...options
      })
      if (retryRes.ok) {
        return retryRes.status === 204 ? (undefined as T) : retryRes.json()
      }
    }
    // Refresh failed — redirect to login
    window.location.href = '/admin/login'
    throw new ApiRequestError(401, { code: 'unauthorized', message: 'Session expired' })
  }

  if (!res.ok) {
    const error: ApiError = await res.json().catch(() => ({
      code: 'unknown',
      message: res.statusText
    }))
    throw new ApiRequestError(res.status, error)
  }

  return res.status === 204 ? (undefined as T) : res.json()
}

export const api = {
  get: <T>(path: string) => request<T>(path),
  post: <T>(path: string, body?: unknown) =>
    request<T>(path, { method: 'POST', body: body ? JSON.stringify(body) : undefined }),
  put: <T>(path: string, body?: unknown) =>
    request<T>(path, { method: 'PUT', body: body ? JSON.stringify(body) : undefined }),
  delete: <T>(path: string) => request<T>(path, { method: 'DELETE' })
}
```

- [ ] **Step 4: Create auth API module**

```typescript
// src/lib/api/auth.ts
import { api } from './client'

export interface AuthUser {
  username: string
  isAdmin: boolean
}

export async function exchange(): Promise<AuthUser> {
  return api.post<AuthUser>('/api/v1/auth/exchange')
}

export async function me(): Promise<AuthUser> {
  return api.get<AuthUser>('/api/v1/auth/me')
}
```

- [ ] **Step 5: Create format utils**

```typescript
// src/lib/utils/format.ts
export function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`
}

export function formatCpu(fraction: number): string {
  return `${(fraction * 100).toFixed(1)}%`
}

export function formatUptime(seconds: number): string {
  const days = Math.floor(seconds / 86400)
  const hours = Math.floor((seconds % 86400) / 3600)
  const mins = Math.floor((seconds % 3600) / 60)
  if (days > 0) return `${days}d ${hours}h`
  if (hours > 0) return `${hours}h ${mins}m`
  return `${mins}m`
}

export function formatPercent(used: number, total: number): number {
  if (total === 0) return 0
  return Math.round((used / total) * 100)
}
```

- [ ] **Step 6: Verify build**

```bash
cd frontend-svelte && npm run build
```

- [ ] **Step 7: Commit**

```bash
git add frontend-svelte/src/lib/
git commit -m "feat(frontend): add TypeScript types, API client, and format utils"
```

---

### Task 9: Auth store + Theme store

**Files:**
- Create: `frontend-svelte/src/lib/stores/auth.svelte.ts`
- Create: `frontend-svelte/src/lib/stores/theme.svelte.ts`

- [ ] **Step 1: Create auth store**

```typescript
// src/lib/stores/auth.svelte.ts
import { exchange, me } from '$lib/api/auth'

interface AuthState {
  username: string
  isAdmin: boolean
  initialized: boolean
}

function createAuthStore() {
  let state = $state<AuthState>({
    username: '',
    isAdmin: false,
    initialized: false
  })

  return {
    get username() { return state.username },
    get isAdmin() { return state.isAdmin },
    get initialized() { return state.initialized },

    async exchange() {
      try {
        const user = await exchange()
        state = { username: user.username, isAdmin: user.isAdmin, initialized: true }
      } catch {
        state = { username: '', isAdmin: false, initialized: true }
        window.location.href = '/admin/login'
      }
    },

    async refresh() {
      try {
        const user = await me()
        state = { ...state, username: user.username, isAdmin: user.isAdmin }
      } catch {
        // Token expired, exchange will handle redirect
      }
    },

    clear() {
      state = { username: '', isAdmin: false, initialized: true }
    }
  }
}

export const auth = createAuthStore()
```

- [ ] **Step 2: Create theme store**

```typescript
// src/lib/stores/theme.svelte.ts
type Theme = 'light' | 'dark'

function createThemeStore() {
  let theme = $state<Theme>('light')

  return {
    get current() { return theme },
    get isDark() { return theme === 'dark' },

    init() {
      if (typeof window === 'undefined') return
      const saved = localStorage.getItem('pvmss-theme') as Theme | null
      const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches
      theme = saved ?? (prefersDark ? 'dark' : 'light')
      this.apply()
    },

    toggle() {
      theme = theme === 'light' ? 'dark' : 'light'
      localStorage.setItem('pvmss-theme', theme)
      this.apply()
    },

    apply() {
      if (typeof document === 'undefined') return
      document.documentElement.classList.toggle('dark', theme === 'dark')
    }
  }
}

export const themeStore = createThemeStore()
```

- [ ] **Step 3: Commit**

```bash
git add frontend-svelte/src/lib/stores/
git commit -m "feat(frontend): add auth and theme stores (Svelte 5 runes)"
```

---

### Task 10: Reusable business components

**Files:**
- Create: `frontend-svelte/src/lib/components/layout/AppShell.svelte`
- Create: `frontend-svelte/src/lib/components/layout/AdminSidebar.svelte`
- Create: `frontend-svelte/src/lib/components/layout/PageHeader.svelte`
- Create: `frontend-svelte/src/lib/components/layout/ThemeToggle.svelte`
- Create: `frontend-svelte/src/lib/components/data/DataTable.svelte`
- Create: `frontend-svelte/src/lib/components/data/ResourceCard.svelte`
- Create: `frontend-svelte/src/lib/components/data/StatusBadge.svelte`
- Create: `frontend-svelte/src/lib/components/data/EmptyState.svelte`
- Create: `frontend-svelte/src/lib/components/data/LoadingSkeleton.svelte`
- Create: `frontend-svelte/src/lib/components/forms/ConfirmDialog.svelte`
- Create: `frontend-svelte/src/lib/components/forms/TagInput.svelte`
- Create: `frontend-svelte/src/lib/components/feedback/ErrorBanner.svelte`

This task builds the full component library. Each component uses:
- Svelte 5 syntax (`$props()`, snippets)
- shadcn-svelte primitives as building blocks
- Strict TypeScript props
- No data fetching — pure presentation

- [ ] **Step 1: Build layout components**

`AppShell`: takes a sidebar snippet and a children snippet. Renders a full-width layout with a top navbar (app name, ThemeToggle, user info) and a sidebar + main content area.

`AdminSidebar`: takes `items: {href, icon, label}[]` and the current path from `$page.url.pathname`. Renders shadcn Sidebar with Phosphor icons and active state highlighting. Items:
- Dashboard (House), Nodes (HardDrives), Storage (Database), VMs (Desktop), User Pools (UsersThree), Tags (Tag), Limits (Sliders), Network (WifiHigh), Cloud-Init (Cloud), ISO (DiscAlbum), App Info (Info)

`PageHeader`: takes `title`, `icon` (Phosphor component), optional `actions` snippet. Renders a header row.

`ThemeToggle`: uses `themeStore.toggle()`. Shows Sun/Moon icon.

- [ ] **Step 2: Build data display components**

`DataTable<T>`: generic sortable table. Props: `data: T[]`, `columns: Column<T>[]`, `loading: boolean`, `emptyMessage: string`, `onRowClick`. Uses shadcn Table underneath. Columns can have a `render` snippet for custom cell content (StatusBadge, progress bars). Sort by clicking column headers.

`ResourceCard`: props `title`, `value`, `icon` (Phosphor component), `subtitle`, `trend` (optional up/down indicator). Uses shadcn Card.

`StatusBadge`: props `status: string`, `variant` derived from status (running→green, stopped→red, paused→yellow, etc). Uses shadcn Badge.

`EmptyState`: props `icon`, `title`, `description`, optional `action` snippet (button). Centered layout with muted text.

`LoadingSkeleton`: props `rows: number`, `variant: 'table' | 'card' | 'form'`. Uses shadcn Skeleton.

- [ ] **Step 3: Build form components**

`ConfirmDialog`: props `open: boolean`, `title`, `description`, `confirmLabel`, `variant: 'destructive' | 'default'`, events `onConfirm`, `onCancel`. Uses shadcn Dialog.

`TagInput`: props `tags: string[]`, `placeholder`, events `onAdd(tag)`, `onRemove(tag)`. Renders chips with X buttons + input field.

- [ ] **Step 4: Build feedback component**

`ErrorBanner`: props `error: ApiRequestError | null`, `onRetry`. Shows alert with error message and retry button. Dismissible.

- [ ] **Step 5: Verify build**

```bash
cd frontend-svelte && npm run build
```

- [ ] **Step 6: Commit**

```bash
git add frontend-svelte/src/lib/components/
git commit -m "feat(frontend): add reusable component library (layout, data, forms, feedback)"
```

---

## Chunk 3: Admin API Modules + Admin Pages

### Task 11: Admin API modules (frontend)

**Files:**
- Create: `frontend-svelte/src/lib/api/admin/nodes.ts`
- Create: `frontend-svelte/src/lib/api/admin/storage.ts`
- Create: `frontend-svelte/src/lib/api/admin/vms.ts`
- Create: `frontend-svelte/src/lib/api/admin/userpool.ts`
- Create: `frontend-svelte/src/lib/api/admin/tags.ts`
- Create: `frontend-svelte/src/lib/api/admin/limits.ts`
- Create: `frontend-svelte/src/lib/api/admin/vmbr.ts`
- Create: `frontend-svelte/src/lib/api/admin/cloudinit.ts`
- Create: `frontend-svelte/src/lib/api/admin/iso.ts`
- Create: `frontend-svelte/src/lib/api/admin/appinfo.ts`

Each module is a thin typed wrapper around `api.get/post/put/delete`.

- [ ] **Step 1: Create all admin API modules**

Pattern for each:
```typescript
// src/lib/api/admin/nodes.ts
import { api } from '$lib/api/client'
import type { Node } from '$lib/types/admin'

export function getNodes(): Promise<Node[]> {
  return api.get('/api/v1/admin/nodes')
}
```

Modules with mutations follow the same pattern:
```typescript
// src/lib/api/admin/tags.ts
import { api } from '$lib/api/client'
import type { Tag } from '$lib/types/admin'

export function getTags(): Promise<Tag[]> {
  return api.get('/api/v1/admin/tags')
}
export function createTag(name: string): Promise<void> {
  return api.post('/api/v1/admin/tags', { name })
}
export function deleteTag(name: string): Promise<void> {
  return api.delete(`/api/v1/admin/tags/${encodeURIComponent(name)}`)
}
```

- [ ] **Step 2: Verify build**

```bash
cd frontend-svelte && npm run build
```

- [ ] **Step 3: Commit**

```bash
git add frontend-svelte/src/lib/api/admin/
git commit -m "feat(frontend): add typed admin API modules"
```

---

### Task 12: Root layout + admin layout + routing

**Files:**
- Modify: `frontend-svelte/src/routes/+layout.svelte`
- Create: `frontend-svelte/src/routes/+layout.ts`
- Modify: `frontend-svelte/src/routes/+page.svelte`
- Create: `frontend-svelte/src/routes/admin/+layout.svelte`
- Create: `frontend-svelte/src/routes/admin/+layout.ts`

- [ ] **Step 1: Create root layout**

```svelte
<!-- src/routes/+layout.svelte -->
<script lang="ts">
  import { onMount } from 'svelte'
  import { auth } from '$lib/stores/auth.svelte'
  import { themeStore } from '$lib/stores/theme.svelte'
  import { Toaster } from '$lib/components/ui/sonner'
  import '../app.css'

  let { children } = $props()

  onMount(async () => {
    themeStore.init()
    await auth.exchange()
  })
</script>

{#if auth.initialized}
  {@render children()}
{:else}
  <div class="flex h-screen items-center justify-center">
    <p class="text-muted-foreground">Loading...</p>
  </div>
{/if}
<Toaster />
```

- [ ] **Step 2: Disable SSR (SPA mode)**

```typescript
// src/routes/+layout.ts
export const prerender = false
export const ssr = false
```

- [ ] **Step 3: Create admin layout**

```svelte
<!-- src/routes/admin/+layout.svelte -->
<script lang="ts">
  import { auth } from '$lib/stores/auth.svelte'
  import { goto } from '$app/navigation'
  import { base } from '$app/paths'
  import AppShell from '$lib/components/layout/AppShell.svelte'
  import AdminSidebar from '$lib/components/layout/AdminSidebar.svelte'

  let { children } = $props()

  $effect(() => {
    if (auth.initialized && !auth.isAdmin) {
      window.location.href = '/login'
    }
  })
</script>

{#if auth.isAdmin}
  <AppShell>
    {#snippet sidebar()}
      <AdminSidebar />
    {/snippet}
    {@render children()}
  </AppShell>
{/if}
```

- [ ] **Step 4: Create stub root page**

```svelte
<!-- src/routes/+page.svelte -->
<script lang="ts">
  import { base } from '$app/paths'
  import { goto } from '$app/navigation'
  import { onMount } from 'svelte'
  // Redirect to admin dashboard (only admin pages available in this phase)
  onMount(() => goto(`${base}/`))
</script>
```

- [ ] **Step 5: Verify build**

```bash
cd frontend-svelte && npm run build
```

- [ ] **Step 6: Commit**

```bash
git add frontend-svelte/src/routes/
git commit -m "feat(frontend): add root layout with auth init + admin layout with guard"
```

---

### Task 13: Admin dashboard page

**Files:**
- Create: `frontend-svelte/src/routes/admin/+page.svelte`

- [ ] **Step 1: Create dashboard page**

The dashboard calls nodes, vms, and storage endpoints in parallel and displays aggregate stats as `ResourceCard` components.

```svelte
<!-- src/routes/admin/+page.svelte -->
<script lang="ts">
  import { onMount } from 'svelte'
  import PageHeader from '$lib/components/layout/PageHeader.svelte'
  import ResourceCard from '$lib/components/data/ResourceCard.svelte'
  import LoadingSkeleton from '$lib/components/data/LoadingSkeleton.svelte'
  import ErrorBanner from '$lib/components/feedback/ErrorBanner.svelte'
  import { getNodes } from '$lib/api/admin/nodes'
  import { getStorages } from '$lib/api/admin/storage'
  import { getAllVMs } from '$lib/api/admin/vms'
  import { formatBytes } from '$lib/utils/format'
  import { House } from 'phosphor-svelte'

  let loading = $state(true)
  let error = $state<Error | null>(null)
  let nodeCount = $state(0)
  let vmCount = $state(0)
  let storageTotal = $state(0)
  let storageUsed = $state(0)

  async function load() {
    loading = true
    error = null
    try {
      const [nodes, vms, storages] = await Promise.all([
        getNodes(), getAllVMs(), getStorages()
      ])
      nodeCount = nodes.length
      vmCount = vms.length
      storageTotal = storages.reduce((s, x) => s + x.total, 0)
      storageUsed = storages.reduce((s, x) => s + x.used, 0)
    } catch (e) {
      error = e as Error
    } finally {
      loading = false
    }
  }

  onMount(load)
</script>

<PageHeader title="Dashboard" icon={House} />

{#if error}
  <ErrorBanner {error} onRetry={load} />
{:else if loading}
  <LoadingSkeleton variant="card" rows={4} />
{:else}
  <div class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
    <ResourceCard title="Nodes" value={String(nodeCount)} subtitle="Active" />
    <ResourceCard title="Virtual Machines" value={String(vmCount)} subtitle="Total" />
    <ResourceCard title="Storage Used" value={formatBytes(storageUsed)} subtitle={`of ${formatBytes(storageTotal)}`} />
    <ResourceCard
      title="Storage Free"
      value={formatBytes(storageTotal - storageUsed)}
      subtitle={`${Math.round(((storageTotal - storageUsed) / storageTotal) * 100)}% available`}
    />
  </div>
{/if}
```

- [ ] **Step 2: Verify build**

```bash
cd frontend-svelte && npm run build
```

- [ ] **Step 3: Commit**

```bash
git add frontend-svelte/src/routes/admin/+page.svelte
git commit -m "feat(frontend): add admin dashboard page with aggregate stats"
```

---

### Task 14: Read-only admin pages (nodes, storage, vmbr, iso, appinfo)

**Files:**
- Create: `frontend-svelte/src/routes/admin/nodes/+page.svelte`
- Create: `frontend-svelte/src/routes/admin/storage/+page.svelte`
- Create: `frontend-svelte/src/routes/admin/vmbr/+page.svelte`
- Create: `frontend-svelte/src/routes/admin/iso/+page.svelte`
- Create: `frontend-svelte/src/routes/admin/appinfo/+page.svelte`

Each follows the same pattern: `onMount` → fetch → render with `DataTable` or `ResourceCard` grid, plus `LoadingSkeleton`, `ErrorBanner`, `EmptyState`.

- [ ] **Step 1: Build nodes page** (ResourceCard grid with CPU/RAM/uptime)
- [ ] **Step 2: Build storage page** (DataTable with name, type, total/used/free, progress bar snippet)
- [ ] **Step 3: Build VMBR page** (DataTable with bridge name, type, active status, node)
- [ ] **Step 4: Build ISO page** (DataTable with name, size formatted, storage)
- [ ] **Step 5: Build appinfo page** (ResourceCard diagnostic cards)
- [ ] **Step 6: Verify build**

```bash
cd frontend-svelte && npm run build
```

- [ ] **Step 7: Commit**

```bash
git add frontend-svelte/src/routes/admin/nodes/ frontend-svelte/src/routes/admin/storage/ frontend-svelte/src/routes/admin/vmbr/ frontend-svelte/src/routes/admin/iso/ frontend-svelte/src/routes/admin/appinfo/
git commit -m "feat(frontend): add read-only admin pages (nodes, storage, vmbr, iso, appinfo)"
```

---

### Task 15: Admin VMs page

**Files:**
- Create: `frontend-svelte/src/routes/admin/vms/+page.svelte`

- [ ] **Step 1: Build VMs page**

DataTable with columns: VMID, name, node, status (StatusBadge snippet), pool, CPU, RAM. Each row has action buttons (start/stop/reboot) shown via dropdown menu. Actions call `vmAction(vmid, action)` → toast on success/error → re-fetch list.

- [ ] **Step 2: Verify build + commit**

```bash
cd frontend-svelte && npm run build
git add frontend-svelte/src/routes/admin/vms/
git commit -m "feat(frontend): add admin VMs page with action buttons"
```

---

### Task 16: Admin tags page

**Files:**
- Create: `frontend-svelte/src/routes/admin/tags/+page.svelte`

- [ ] **Step 1: Build tags page**

Displays tags as chips using `TagInput` component, with VM count per tag. "Create tag" button opens a dialog. Delete with `ConfirmDialog`. After each mutation, re-fetch tags.

- [ ] **Step 2: Verify build + commit**

```bash
cd frontend-svelte && npm run build
git add frontend-svelte/src/routes/admin/tags/
git commit -m "feat(frontend): add admin tags page with create/delete"
```

---

### Task 17: Admin userpool page

**Files:**
- Create: `frontend-svelte/src/routes/admin/userpool/+page.svelte`

- [ ] **Step 1: Build userpool page**

DataTable listing pools with members. "Create Pool" button opens a dialog with fields: pool name, Proxmox username, password. Delete with `ConfirmDialog` (warns about cascading VM cleanup). This is the most complex mutation page — mirrors the existing `admin_userpool.templ` + `user_pool.go` functionality.

- [ ] **Step 2: Verify build + commit**

```bash
cd frontend-svelte && npm run build
git add frontend-svelte/src/routes/admin/userpool/
git commit -m "feat(frontend): add admin userpool page with create/delete"
```

---

### Task 18: Admin limits page

**Files:**
- Create: `frontend-svelte/src/routes/admin/limits/+page.svelte`

- [ ] **Step 1: Build limits page**

Form with shadcn Input fields for: CPU sockets min/max, cores min/max, RAM min/max, disk min/max, max VMs per user, max snapshots per VM, max network cards, max disks per VM. Loads current values from `GET /api/v1/admin/limits`. Save button calls `PUT /api/v1/admin/limits`.

- [ ] **Step 2: Verify build + commit**

```bash
cd frontend-svelte && npm run build
git add frontend-svelte/src/routes/admin/limits/
git commit -m "feat(frontend): add admin limits page with editable form"
```

---

### Task 19: Admin cloudinit page

**Files:**
- Create: `frontend-svelte/src/routes/admin/cloudinit/+page.svelte`

- [ ] **Step 1: Build cloudinit page**

DataTable with columns: name, description, storage, enabled (toggle Switch). "Create Template" button opens a dialog with name, description, storage, YAML content (textarea). Edit opens the same dialog pre-filled. Delete with `ConfirmDialog`. Toggle calls `POST /api/v1/admin/cloudinit/:id/toggle`.

- [ ] **Step 2: Verify build + commit**

```bash
cd frontend-svelte && npm run build
git add frontend-svelte/src/routes/admin/cloudinit/
git commit -m "feat(frontend): add admin cloudinit page with CRUD + toggle"
```

---

## Chunk 4: Integration & Polish

### Task 20: Makefile updates

**Files:**
- Modify: `Makefile`

- [ ] **Step 1: Add frontend targets**

```makefile
frontend-install:
	cd frontend-svelte && npm ci

frontend-build:
	cd frontend-svelte && npm run build

frontend-dev:
	cd frontend-svelte && npm run dev

dev-api:
	cd backend && go run .

dev: frontend-install
	npx concurrently -n "api,svelte" -c "blue,green" "make dev-api" "make frontend-dev"
```

- [ ] **Step 2: Verify `make frontend-build` works**

```bash
make frontend-build
```

- [ ] **Step 3: Commit**

```bash
git add Makefile
git commit -m "chore(makefile): add frontend-svelte build and dev targets"
```

---

### Task 21: End-to-end verification

- [ ] **Step 1: Run all Go tests**

```bash
cd backend && go test ./... -count=1 2>&1 | tail -20
```
Expected: All existing tests pass, all new admin API tests pass.

- [ ] **Step 2: Build the full frontend**

```bash
make frontend-build
```
Expected: `frontend-svelte/build/` directory created with `index.html` and `assets/`

- [ ] **Step 3: Manual smoke test**

```bash
make dev
```

Open `http://localhost:5173/admin/` in a browser. Verify:
- Auth exchange works (redirects to login if not authenticated)
- Admin sidebar shows all 11 items
- Each page loads data correctly
- Dark mode toggle works
- CRUD operations on tags, pools, limits, cloudinit work
- VM actions (start/stop) work
- Toast notifications appear on success/error

- [ ] **Step 4: Verify no regressions on legacy pages**

Open `http://localhost:50000/` — user-facing templ pages should work identically.

- [ ] **Step 5: Final commit if any fixes needed**

```bash
git add -A
git commit -m "fix: address integration issues from smoke test"
```

---

## Dependency Order

```
BACKEND TRACK (can run in parallel with frontend track):
  Task 1 (types)
    → Task 2 + 3 + 4 [parallel] (handlers)
      → Task 5 (route registration + SPA serving)

FRONTEND TRACK (can run in parallel with backend track):
  Task 6 (scaffold) → Task 7 (shadcn)
    → Task 8 (types + client) → Task 9 (stores)
      → Task 10 (components)
        → Task 11 (API modules)
          → Task 12 (layouts)
            → Task 13-19 (pages) [parallel]

INTEGRATION (requires both tracks):
  Task 20 (Makefile) → Task 21 (verification)
```

- Tasks 2, 3, 4 can run in parallel (backend track).
- The entire frontend track (Tasks 6-19) can run in parallel with the backend track (Tasks 1-5).
- Tasks 13-19 (all admin pages) can run in parallel after Task 12.
- Task 20-21 require both tracks to be complete.
