# Server-Side Pagination Implementation Plan

Implement comprehensive server-side pagination for VM listings across both user profile and admin interfaces, using the existing ProxmoxClusterSnapshot cache with data slicing, server-side search/filtering, and sorting capabilities.

## Scope & Requirements

**In Scope:**

- Server-side pagination for user profile VM list (`/profile`)
- Server-side pagination for new admin VM list page (`/admin/vms`)
- Use existing `ProxmoxClusterSnapshot.VMs` cache with Go-based slicing
- Default: 20 items per page, max 100
- Server-side search/filtering only
- Server-side sorting by vmid, name, status, cpu, memory
- No URL query parameter state (not bookmarkable)
- Full i18n support (EN/FR)

**Out of Scope:**

- Client-side pagination/fallback
- URL state persistence
- Real-time updates during pagination navigation
- Advanced filtering beyond current search params

## Architecture Overview

### Backend Flow

1. API receives request with `page`, `limit`, `search`, `sort`, `order` params
2. Handler retrieves VMs from `ProxmoxClusterSnapshot` cache
3. Applies pool filtering (for non-admin users)
4. Applies server-side search/filtering
5. Applies sorting
6. Slices results based on pagination params
7. Returns paginated response with metadata

### Frontend Flow

1. Component maintains local state: `currentPage`, `pageSize`, `searchQuery`, `sortBy`, `sortOrder`
2. On state change, calls API with all params
3. Displays pagination controls using existing bits-ui component
4. Shows search input and sort dropdown
5. Displays VM table with current page results

## Implementation Phases

### Phase 1: Backend API Enhancements

#### 1.1 Update API Types (`backend/api/v1/types.go`)

**Add pagination request/response types:**

```go
// PaginationRequest represents pagination parameters
type PaginationRequest struct {
    Page     int    `json:"page"`     // Default: 1
    Limit    int    `json:"limit"`    // Default: 20, Max: 100
    Search   string `json:"search"`   // Optional search query
    SortBy   string `json:"sort_by"`  // vmid, name, status, cpu, memory
    SortOrder string `json:"sort_order"` // asc, desc
}

// PaginationResponse wraps paginated results with metadata
type PaginationResponse struct {
    Data       interface{} `json:"data"`
    Pagination PaginationMetadata `json:"pagination"`
}

// PaginationMetadata provides pagination information
type PaginationMetadata struct {
    Total      int `json:"total"`
    Page       int `json:"page"`
    Limit      int `json:"limit"`
    TotalPages int `json:"total_pages"`
    HasNext    bool `json:"has_next"`
    HasPrev    bool `json:"has_prev"`
}

// VMListPaginatedResponse is the paginated VM list response
type VMListPaginatedResponse struct {
    VMs        []VMSummary         `json:"vms"`
    Pagination PaginationMetadata `json:"pagination"`
}
```

#### 1.2 Update VM Handler (`backend/api/v1/vms.go`)

**Modify `ListVMs` to support pagination:**

```go
func (h *VMHandler) ListVMs(w http.ResponseWriter, r *http.Request) {
    if h.isOffline() {
        writeJSON(w, VMListPaginatedResponse{
            VMs: []VMSummary{},
            Pagination: PaginationMetadata{
                Total: 0, Page: 1, Limit: 20, TotalPages: 0,
                HasNext: false, HasPrev: false,
            },
        })
        return
    }

    // Parse pagination parameters
    page := parseIntParam(r.URL.Query().Get("page"), 1)
    limit := parseIntParam(r.URL.Query().Get("limit"), 20)
    if limit > 100 {
        limit = 100
    }
    if limit < 1 {
        limit = 20
    }
    search := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("search")))
    sortBy := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("sort_by")))
    sortOrder := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("sort_order")))

    // Default sort by vmid ascending
    if sortBy == "" {
        sortBy = "vmid"
    }
    if sortOrder == "" {
        sortOrder = "asc"
    }

    username := usernameFromCtx(r)
    isAdmin := isAdminFromCtx(r)

    // Get VMs from cache
    snapshot := h.state.GetProxmoxSnapshot()
    var summaries []VMSummary
    
    if snapshot == nil {
        // Fallback to live API if cache unavailable
        client, err := restyClient()
        if err != nil {
            errInternal(w)
            return
        }
        ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
        defer cancel()
        allVMs, err := proxmox.GetVMsResty(ctx, client)
        if err != nil {
            errInternal(w)
            return
        }
        summaries = processVMList(allVMs, username, isAdmin, search, sortBy, sortOrder)
    } else {
        // Use cached VMs
        summaries = processCachedVMs(snapshot.VMs, username, isAdmin, search, sortBy, sortOrder)
    }

    // Apply pagination
    total := len(summaries)
    totalPages := (total + limit - 1) / limit
    if totalPages == 0 {
        totalPages = 1
    }
    
    if page < 1 {
        page = 1
    }
    if page > totalPages {
        page = totalPages
    }

    offset := (page - 1) * limit
    end := offset + limit
    if end > total {
        end = total
    }
    
    var pagedSummaries []VMSummary
    if offset < total {
        pagedSummaries = summaries[offset:end]
    } else {
        pagedSummaries = []VMSummary{}
    }

    response := VMListPaginatedResponse{
        VMs: pagedSummaries,
        Pagination: PaginationMetadata{
            Total:      total,
            Page:       page,
            Limit:      limit,
            TotalPages: totalPages,
            HasNext:    page < totalPages,
            HasPrev:    page > 1,
        },
    }

    writeJSON(w, response)
}

// Helper function to parse int parameters with defaults
func parseIntParam(s string, defaultValue int) int {
    if s == "" {
        return defaultValue
    }
    i, err := strconv.Atoi(s)
    if err != nil || i < 0 {
        return defaultValue
    }
    return i
}

// Helper function to process cached VMs
func processCachedVMs(vms []state.SnapshotVM, username string, isAdmin bool, 
    search, sortBy, sortOrder string) []VMSummary {
    
    summaries := make([]VMSummary, 0, len(vms))
    
    for _, vm := range vms {
        // Pool filter for non-admin users
        if !isAdmin && username != "" {
            // Note: SnapshotVM doesn't have pool info, need to fetch from cache or API
            // For now, skip pool filter on cached data - will need enhancement
        }
        
        // Tag filter
        if !hasTag(vm.Tags, "pvmss") {
            continue
        }
        
        // Search filter
        if search != "" {
            vmidStr := strconv.Itoa(vm.VMID)
            matched := strings.Contains(strings.ToLower(vm.Name), search) ||
                strings.Contains(vmidStr, search) ||
                containsTagSubstring(vm.Tags, search)
            if !matched {
                continue
            }
        }
        
        summaries = append(summaries, snapshotVMToSummary(vm))
    }
    
    // Apply sorting
    sortVMs(summaries, sortBy, sortOrder)
    
    return summaries
}

// Convert SnapshotVM to VMSummary
func snapshotVMToSummary(vm state.SnapshotVM) VMSummary {
    return VMSummary{
        VMID:     vm.VMID,
        Name:     vm.Name,
        Node:     vm.Node,
        Status:   vm.Status,
        CPU:      0, // Not available in SnapshotVM
        CPUs:     vm.Cores,
        MemMB:    vm.MemoryMB,
        MaxMemMB: vm.MemoryMB,
        DiskMB:   0, // Not available in SnapshotVM
        Uptime:   0, // Not available in SnapshotVM
        Tags:     vm.Tags,
    }
}

// Sort VMs by specified field
func sortVMs(vms []VMSummary, sortBy, sortOrder string) {
    ascending := sortOrder == "asc"
    
    switch sortBy {
    case "vmid":
        if ascending {
            slices.SortFunc(vms, func(a, b VMSummary) int { return a.VMID - b.VMID })
        } else {
            slices.SortFunc(vms, func(a, b VMSummary) int { return b.VMID - a.VMID })
        }
    case "name":
        if ascending {
            slices.SortFunc(vms, func(a, b VMSummary) int { 
                return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
            })
        } else {
            slices.SortFunc(vms, func(a, b VMSummary) int { 
                return strings.Compare(strings.ToLower(b.Name), strings.ToLower(a.Name))
            })
        }
    case "status":
        if ascending {
            slices.SortFunc(vms, func(a, b VMSummary) int { 
                return strings.Compare(a.Status, b.Status)
            })
        } else {
            slices.SortFunc(vms, func(a, b VMSummary) int { 
                return strings.Compare(b.Status, a.Status)
            })
        }
    case "cpu":
        if ascending {
            slices.SortFunc(vms, func(a, b VMSummary) int { 
                if a.CPU < b.CPU { return -1 }
                if a.CPU > b.CPU { return 1 }
                return 0
            })
        } else {
            slices.SortFunc(vms, func(a, b VMSummary) int { 
                if a.CPU > b.CPU { return -1 }
                if a.CPU < b.CPU { return 1 }
                return 0
            })
        }
    case "memory":
        if ascending {
            slices.SortFunc(vms, func(a, b VMSummary) int { 
                if a.MaxMemMB < b.MaxMemMB { return -1 }
                if a.MaxMemMB > b.MaxMemMB { return 1 }
                return 0
            })
        } else {
            slices.SortFunc(vms, func(a, b VMSummary) int { 
                if a.MaxMemMB > b.MaxMemMB { return -1 }
                if a.MaxMemMB < b.MaxMemMB { return 1 }
                return 0
            })
        }
    default:
        // Default sort by vmid
        if ascending {
            slices.SortFunc(vms, func(a, b VMSummary) int { return a.VMID - b.VMID })
        } else {
            slices.SortFunc(vms, func(a, b VMSummary) int { return b.VMID - a.VMID })
        }
    }
}
```

#### 1.3 Update Admin VM Handler (`backend/api/v1/admin_vms.go`)

**Modify `ListAllVMs` to support pagination:**

```go
// ListAllVMsPaginated handles GET /api/v1/admin/vms with pagination
func (h *AdminVMsAPIHandler) ListAllVMsPaginated(w http.ResponseWriter, r *http.Request) {
    if h.state.IsOfflineMode() {
        writeJSON(w, VMListPaginatedResponse{
            VMs: []VMSummary{},
            Pagination: PaginationMetadata{
                Total: 0, Page: 1, Limit: 20, TotalPages: 0,
                HasNext: false, HasPrev: false,
            },
        })
        return
    }

    // Parse pagination parameters (same as user endpoint)
    page := parseIntParam(r.URL.Query().Get("page"), 1)
    limit := parseIntParam(r.URL.Query().Get("limit"), 20)
    if limit > 100 {
        limit = 100
    }
    if limit < 1 {
        limit = 20
    }
    search := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("search")))
    sortBy := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("sort_by")))
    sortOrder := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("sort_order")))

    if sortBy == "" {
        sortBy = "vmid"
    }
    if sortOrder == "" {
        sortOrder = "asc"
    }

    // Get VMs from cache
    snapshot := h.state.GetProxmoxSnapshot()
    var summaries []VMSummary
    
    if snapshot == nil {
        // Fallback to live API
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
        for _, vm := range vms {
            if !hasTag(vm.Tags, "pvmss") {
                continue
            }
            if search != "" && !matchesSearch(vm, search) {
                continue
            }
            summaries = append(summaries, vmToSummary(vm))
        }
    } else {
        // Use cached VMs
        for _, vm := range snapshot.VMs {
            if !hasTag(vm.Tags, "pvmss") {
                continue
            }
            if search != "" && !matchesCachedSearch(vm, search) {
                continue
            }
            summaries = append(summaries, snapshotVMToSummary(vm))
        }
    }

    // Apply sorting
    sortVMs(summaries, sortBy, sortOrder)

    // Apply pagination (same logic as user endpoint)
    total := len(summaries)
    totalPages := (total + limit - 1) / limit
    if totalPages == 0 {
        totalPages = 1
    }
    
    if page < 1 {
        page = 1
    }
    if page > totalPages {
        page = totalPages
    }

    offset := (page - 1) * limit
    end := offset + limit
    if end > total {
        end = total
    }
    
    var pagedSummaries []VMSummary
    if offset < total {
        pagedSummaries = summaries[offset:end]
    } else {
        pagedSummaries = []VMSummary{}
    }

    response := VMListPaginatedResponse{
        VMs: pagedSummaries,
        Pagination: PaginationMetadata{
            Total:      total,
            Page:       page,
            Limit:      limit,
            TotalPages: totalPages,
            HasNext:    page < totalPages,
            HasPrev:    page > 1,
        },
    }

    writeJSON(w, response)
}

// Helper: match search on cached VM
func matchesCachedSearch(vm state.SnapshotVM, search string) bool {
    vmidStr := strconv.Itoa(vm.VMID)
    return strings.Contains(strings.ToLower(vm.Name), search) ||
        strings.Contains(vmidStr, search) ||
        containsTagSubstring(vm.Tags, search)
}
```

#### 1.4 Register New Route (`backend/api/v1/router.go`)

**Add new paginated endpoint while keeping old one for backward compatibility:**

```go
// Keep existing route for backward compatibility
router.GET("/api/v1/admin/vms", adminVMsHandler.ListAllVMs)

// Add new paginated endpoint
router.GET("/api/v1/admin/vms/paginated", adminVMsHandler.ListAllVMsPaginated)
```

### Phase 2: Frontend User Profile Page

#### 2.1 Update API Client (`frontend/src/lib/api/vms.ts`)

```typescript
export interface PaginatedVMListResponse {
  vms: VMSummary[];
  pagination: {
    total: number;
    page: number;
    limit: number;
    total_pages: number;
    has_next: boolean;
    has_prev: boolean;
  };
}

export interface VMPaginationParams {
  page?: number;
  limit?: number;
  search?: string;
  sort_by?: string;
  sort_order?: string;
}

export async function getVMsPaginated(params: VMPaginationParams = {}): Promise<PaginatedVMListResponse> {
  const qs = new URLSearchParams();
  if (params.page) qs.set("page", params.page.toString());
  if (params.limit) qs.set("limit", params.limit.toString());
  if (params.search) qs.set("search", params.search);
  if (params.sort_by) qs.set("sort_by", params.sort_by);
  if (params.sort_order) qs.set("sort_order", params.sort_order);
  
  const query = qs.toString();
  return api.get<PaginatedVMListResponse>(
    `/api/v1/vms${query ? "?" + query : ""}`
  );
}
```

#### 2.2 Update Profile Page (`frontend/src/routes/(app)/profile/+page.svelte`)

Replace current VM list section with paginated version including:

- Pagination state management (currentPage, pageSize, totalVMs, totalPages, hasNext, hasPrev)
- Search and sort state (searchQuery, sortBy, sortOrder)
- Search input with debouncing (300ms)
- Sort dropdown with toggle button
- Page size selector (10, 20, 50, 100)
- VM table with sortable headers
- Pagination controls using existing bits-ui component
- Loading, error, and empty states

### Phase 3: Frontend Admin VM List Page

#### 3.1 Create Admin VMs API Client (`frontend/src/lib/api/admin/vms.ts`)

```typescript
export interface AdminVMPaginationParams {
  page?: number;
  limit?: number;
  search?: string;
  sort_by?: string;
  sort_order?: string;
}

export async function getAllVMsPaginated(params: AdminVMPaginationParams = {}): Promise<PaginatedVMListResponse> {
  const qs = new URLSearchParams();
  if (params.page) qs.set("page", params.page.toString());
  if (params.limit) qs.set("limit", params.limit.toString());
  if (params.search) qs.set("search", params.search);
  if (params.sort_by) qs.set("sort_by", params.sort_by);
  if (params.sort_order) qs.set("sort_order", params.sort_order);
  
  const query = qs.toString();
  return api.get<PaginatedVMListResponse>(
    `/api/v1/admin/vms/paginated${query ? "?" + query : ""}`
  );
}
```

#### 3.2 Create Admin VM List Page (`frontend/src/routes/admin/vms/+page.svelte`)

Create new page with same pagination UI as profile page but:

- Uses admin API endpoint
- Shows all VMs (not pool-filtered)
- Includes delete action button
- Admin-specific translations

#### 3.3 Add Admin Navigation Link

Add link to new VM list page in admin sidebar/layout.

### Phase 4: Internationalization

Add translations for:

- Search placeholders
- Sort labels (vmid, name, status, cpu, memory)
- Sort order labels (ascending, descending)
- Table headers
- Action buttons
- Empty state messages
- Pagination info text

Add to both `backend/i18n/active.en.toml` and `backend/i18n/active.fr.toml`.

### Phase 5: Testing

#### 5.1 Backend Tests

Create `backend/api/v1/vms_test.go`:

- Test pagination parameters (page, limit validation)
- Test search filtering
- Test sorting for all fields
- Test cache fallback
- Test pool filtering
- Test edge cases (empty results, single page, last page)

#### 5.2 Frontend Tests

Test pagination controls, search debouncing, sort toggling, page size changes.

#### 5.3 Integration Tests

Test full flow from frontend to backend with various pagination scenarios.

### Phase 6: Cache Enhancement (Future Enhancement)

**Note:** The current `SnapshotVM` structure doesn't include pool information or live CPU/memory stats. For production use, consider:

1. Add pool field to `SnapshotVM` structure
2. Add live CPU/memory fields to `SnapshotVM` (updated via guest agent or periodic polling)
3. Implement cache invalidation strategy for VM actions
4. Consider adding a "last updated" timestamp to detect stale data

## File Changes Summary

### Backend Files to Modify

1. `backend/api/v1/types.go` - Add pagination types
2. `backend/api/v1/vms.go` - Update ListVMs with pagination
3. `backend/api/v1/admin_vms.go` - Add ListAllVMsPaginated
4. `backend/api/v1/router.go` - Register new route
5. `backend/i18n/active.en.toml` - Add translations
6. `backend/i18n/active.fr.toml` - Add translations

### Frontend Files to Modify

1. `frontend/src/lib/api/vms.ts` - Add paginated API function
2. `frontend/src/routes/(app)/profile/+page.svelte` - Replace with paginated version
3. `frontend/src/lib/api/admin/vms.ts` - Add paginated API function
4. `frontend/src/routes/admin/vms/+page.svelte` - Create new page (NEW FILE)
5. `frontend/src/routes/admin/+layout.svelte` - Add navigation link

### Test Files to Create

1. `backend/api/v1/vms_test.go` - Backend pagination tests

## Implementation Order

1. **Phase 1**: Backend API types and handler modifications
2. **Phase 2**: Frontend user profile page updates
3. **Phase 3**: Frontend admin VM list page creation
4. **Phase 4**: Internationalization
5. **Phase 5**: Testing
6. **Phase 6**: Cache enhancements (future, optional)

## Acceptance Criteria

- [ ] User profile VM list shows pagination controls when > 20 VMs
- [ ] Admin VM list page shows pagination controls when > 20 VMs
- [ ] Page size can be changed (10, 20, 50, 100)
- [ ] Search filters VMs server-side
- [ ] Sorting works for vmid, name, status, cpu, memory
- [ ] Sort order can be toggled (asc/desc)
- [ ] Navigation between pages works correctly
- [ ] Total count and page info display correctly
- [ ] Empty states show appropriate messages
- [ ] Loading states display during data fetch
- [ ] Error states display on API failure
- [ ] All text is internationalized (EN/FR)
- [ ] Cache fallback works when snapshot unavailable
- [ ] Pagination resets to page 1 on new search or page size change
- [ ] Page adjusts automatically if current page exceeds total after deletions
- [ ] Backend tests pass
- [ ] Frontend components render without errors

## Performance Considerations

- Cache refresh interval: 45 seconds (existing)
- Pagination reduces frontend memory usage significantly
- Server-side slicing is O(1) after filtering/sorting
- Search debouncing (300ms) reduces API calls
- No URL state means no browser history pollution

## Known Limitations

1. **Pool filtering on cached data**: `SnapshotVM` doesn't include pool information, so non-admin users may see incorrect pool filtering when using cache. Workaround: Use live API for non-admin users or enhance cache structure.
2. **Live stats missing**: `SnapshotVM` doesn't include live CPU/memory/uptime. Current implementation shows 0 for these fields when using cache.
3. **No URL state**: Pagination state is not bookmarkable or shareable.
4. **Cache staleness**: 45-second refresh interval means data may be slightly stale.

## Migration Notes

- Old non-paginated endpoints remain for backward compatibility
- Frontend switches to new paginated endpoints immediately
- No database migration required
- No breaking changes to existing API contracts
