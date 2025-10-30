package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"pvmss/i18n"
	"pvmss/logger"
	"pvmss/proxmox"
	"pvmss/security"
	"pvmss/state"

	"github.com/julienschmidt/httprouter"
)

// SearchOptimizedHandler handles search requests with optimized cluster performance
type SearchOptimizedHandler struct {
	stateManager state.StateManager
}

// NewSearchOptimizedHandler creates a new instance of SearchOptimizedHandler
func NewSearchOptimizedHandler(sm state.StateManager) *SearchOptimizedHandler {
	return &SearchOptimizedHandler{stateManager: sm}
}

// RegisterRoutes registers search routes
func (h *SearchOptimizedHandler) RegisterRoutes(router *httprouter.Router) {
	log := logger.Get().With().
		Str("component", "SearchOptimizedHandler").
		Str("function", "RegisterRoutes").
		Logger()

	if router == nil {
		log.Error().Msg("Router is nil, cannot register search routes")
		return
	}

	log.Debug().Msg("Registering optimized search routes")

	router.GET("/search", RequireAuthHandle(h.SearchPageHandler))
	router.POST("/search", SecureFormHandler("Search",
		RequireAuthHandle(h.SearchPageHandler),
	))

	log.Info().
		Strs("routes", []string{"GET /search", "POST /search"}).
		Msg("Optimized search routes registered successfully")
}

// SearchPageHandler handles both GET and POST requests for search page with optimizations
func (h *SearchOptimizedHandler) SearchPageHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("SearchPageHandler", r)

	// Get user info from session
	username := ""
	isAdmin := false
	if sessionManager := security.GetSession(r); sessionManager != nil {
		if user, ok := sessionManager.Get(r.Context(), "username").(string); ok {
			username = user
		}
		if admin, ok := sessionManager.Get(r.Context(), "is_admin").(bool); ok {
			isAdmin = admin
		}
	}

	log.Info().
		Str("username", username).
		Bool("is_admin", isAdmin).
		Msg("Optimized search request started")

	data := map[string]interface{}{
		"TitleKey":        "Search.Title",
		"Lang":            i18n.GetLanguage(r),
		"IsAuthenticated": true,
		"Results":         []map[string]interface{}{},
		"FormData":        map[string]string{},
		"Query":           "",
		"NoResults":       false,
	}

	// Handle POST search requests
	if r.Method == "POST" {
		if err := r.ParseForm(); err != nil {
			log.Error().Err(err).Msg("Failed to parse form")
			data["Error"] = "Invalid form data"
			renderTemplateInternal(w, r, "search", data)
			return
		}

		vmidQuery := strings.TrimSpace(r.FormValue("vmid"))
		nameQuery := strings.TrimSpace(r.FormValue("name"))

		log.Info().
			Str("vmid_query", vmidQuery).
			Str("name_query", nameQuery).
			Msg("Processing optimized search query")

		// Build query display string
		var queryParts []string
		if vmidQuery != "" {
			queryParts = append(queryParts, "VMID: "+vmidQuery)
		}
		if nameQuery != "" {
			queryParts = append(queryParts, "Name: "+nameQuery)
		}
		queryDisplay := strings.Join(queryParts, ", ")
		if queryDisplay == "" {
			queryDisplay = "All VMs"
		}

		data["Query"] = queryDisplay
		data["FormData"] = map[string]string{
			"vmid": vmidQuery,
			"name": nameQuery,
		}

		// Get Proxmox client
		client := h.stateManager.GetProxmoxClient()
		if client == nil {
			log.Error().Msg("Proxmox client not available")
			data["Error"] = "Proxmox connection not available"
			renderTemplateInternal(w, r, "search", data)
			return
		}

		// Create context with timeout (shorter for better UX)
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()

		// Perform optimized search
		results, err := h.searchVMsOptimized(ctx, client, vmidQuery, nameQuery, username, isAdmin)
		if err != nil {
			log.Error().Err(err).Msg("Optimized search failed")
			data["Error"] = fmt.Sprintf("Search failed: %v", err)
			renderTemplateInternal(w, r, "search", data)
			return
		}

		if len(results) > 0 {
			data["Results"] = results
		} else {
			data["NoResults"] = true
		}

		log.Info().
			Int("results_count", len(results)).
			Msg("Optimized search completed successfully")
	}

	renderTemplateInternal(w, r, "search", data)
}

// searchVMsOptimized performs VM search with batch API calls and concurrent processing
func (h *SearchOptimizedHandler) searchVMsOptimized(ctx context.Context, client proxmox.ClientInterface, vmidQuery, nameQuery, username string, isAdmin bool) ([]map[string]interface{}, error) {
	log := logger.Get().With().
		Str("function", "searchVMsOptimized").
		Str("vmid_query", vmidQuery).
		Str("name_query", nameQuery).
		Str("username", username).
		Bool("is_admin", isAdmin).
		Logger()

	// Create resty client
	restyClient, err := getDefaultRestyClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create resty client: %w", err)
	}

	// Get all VMs from Proxmox using resty
	allVMs, err := proxmox.GetVMsResty(ctx, restyClient)
	if err != nil {
		return nil, fmt.Errorf("failed to get VMs (resty): %w", err)
	}

	log.Info().Int("total_vms", len(allVMs)).Msg("Retrieved all VMs (resty)")

	// For non-admin users, get their pool VMs
	var userPoolVMIDs map[int]bool
	if !isAdmin && username != "" {
		poolName := "pvmss_" + username
		userPoolVMIDs = h.getPoolVMIDs(ctx, client, poolName)
		log.Info().
			Str("pool", poolName).
			Int("pool_vm_count", len(userPoolVMIDs)).
			Msg("Retrieved user pool VMs")
	}

	// Filter VMs first before getting configs (reduces API calls)
	filteredVMs := []proxmox.VMInfo{}
	lowerVMIDQuery := strings.ToLower(vmidQuery)
	lowerNameQuery := strings.ToLower(nameQuery)

	for _, vm := range allVMs {
		// Convert VM to VMInfo for consistency
		vmInfo := proxmox.VMInfo{
			VMID:     strconv.Itoa(vm.VMID),
			Name:     vm.Name,
			Status:   vm.Status,
			Node:     vm.Node,
			CPU:      vm.CPUs,
			Memory:   vm.MaxMem,
			Disk:     vm.MaxDisk,
			Template: false, // Will be determined from config if needed
		}

		// Check 1: Pool membership for non-admin users
		if !isAdmin && userPoolVMIDs != nil {
			vmidInt, err := strconv.Atoi(vmInfo.VMID)
			if err != nil {
				continue // Skip invalid VMID
			}
			if !userPoolVMIDs[vmidInt] {
				continue // VM not in user's pool
			}
		}

		// Check 2: Match search criteria (if provided) - do this BEFORE getting config
		if vmidQuery != "" || nameQuery != "" {
			vmidStr := strconv.Itoa(vm.VMID)
			vmName := strings.ToLower(vm.Name)

			matchesVMID := lowerVMIDQuery != "" && strings.Contains(vmidStr, lowerVMIDQuery)
			matchesName := lowerNameQuery != "" && strings.Contains(vmName, lowerNameQuery)

			// If both queries provided, match either
			// If only one query provided, must match that one
			if lowerVMIDQuery != "" && lowerNameQuery != "" {
				if !matchesVMID && !matchesName {
					continue // Doesn't match either
				}
			} else if lowerVMIDQuery != "" {
				if !matchesVMID {
					continue
				}
			} else if lowerNameQuery != "" {
				if !matchesName {
					continue
				}
			}
		}

		// VM passed initial filters, add to filtered list
		filteredVMs = append(filteredVMs, vmInfo)

		// Limit early to avoid unnecessary config calls
		if len(filteredVMs) >= 50 {
			break
		}
	}

	log.Info().
		Int("filtered_vms", len(filteredVMs)).
		Int("original_vms", len(allVMs)).
		Msg("VMs filtered before config check")

	// BATCH: Get configs only for filtered VMs using concurrent goroutines
	vmConfigs := make(map[int]map[string]interface{})
	var wg sync.WaitGroup
	var mu sync.Mutex

	// Use semaphore to limit concurrent API calls (prevents overwhelming Proxmox)
	semaphore := make(chan struct{}, 10) // Max 10 concurrent config calls

	for _, vm := range filteredVMs {
		wg.Add(1)
		go func(vmInfo proxmox.VMInfo) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			vmidInt, err := strconv.Atoi(vmInfo.VMID)
			if err != nil {
				log.Debug().Err(err).Str("vmid", vmInfo.VMID).Msg("Invalid VMID, skipping")
				return
			}
			cfg, err := proxmox.GetVMConfigWithContext(ctx, client, vmInfo.Node, vmidInt)
			if err != nil {
				log.Debug().Err(err).Int("vmid", vmidInt).Msg("Failed to get VM config, skipping")
				return
			}

			// Check for pvmss tag
			if !h.hasTag(cfg, "pvmss") {
				log.Debug().
					Int("vmid", vmidInt).
					Str("name", vmInfo.Name).
					Msg("VM does not have pvmss tag, skipping")
				return
			}

			// Store config
			mu.Lock()
			vmConfigs[vmidInt] = cfg
			mu.Unlock()
		}(vm)
	}

	wg.Wait()

	log.Info().
		Int("vms_with_pvmss_tag", len(vmConfigs)).
		Msg("VMs with pvmss tag identified")

	// Build final results
	results := []map[string]interface{}{}
	for _, vm := range filteredVMs {
		vmidInt, err := strconv.Atoi(vm.VMID)
		if err != nil {
			continue // Skip invalid VMID
		}
		cfg, hasConfig := vmConfigs[vmidInt]
		if !hasConfig {
			continue // Skip VMs without pvmss tag or config errors
		}

		// Build result
		description := ""
		if desc, ok := cfg["description"].(string); ok {
			description = desc
		}

		status := vm.Status
		if status == "" {
			status = "unknown"
		}

		results = append(results, map[string]interface{}{
			"vmid":        vm.VMID,
			"name":        vm.Name,
			"description": description,
			"node":        vm.Node,
			"status":      strings.ToLower(status),
		})

		// Limit results to 50
		if len(results) >= 50 {
			break
		}
	}

	log.Info().
		Int("results_count", len(results)).
		Int("vms_checked", len(allVMs)).
		Int("config_calls_made", len(filteredVMs)).
		Msg("Optimized search filtering completed")

	return results, nil
}

// getPoolVMIDs retrieves VM IDs from a Proxmox pool (same as original)
func (h *SearchOptimizedHandler) getPoolVMIDs(ctx context.Context, client proxmox.ClientInterface, poolName string) map[int]bool {
	log := logger.Get().With().
		Str("function", "getPoolVMIDs").
		Str("pool", poolName).
		Logger()

	vmids := make(map[int]bool)

	var poolResp struct {
		Data struct {
			Members []struct {
				Type     string `json:"type"`
				VMID     int    `json:"vmid"`
				Template int    `json:"template"`
			} `json:"members"`
		} `json:"data"`
	}

	if err := client.GetJSON(ctx, "/pools/"+poolName, &poolResp); err != nil {
		log.Warn().Err(err).Msg("Failed to fetch pool members")
		return vmids
	}

	for _, member := range poolResp.Data.Members {
		if member.Template == 1 || member.VMID <= 0 {
			continue
		}
		if strings.EqualFold(member.Type, "qemu") {
			vmids[member.VMID] = true
		}
	}

	log.Debug().Int("vm_count", len(vmids)).Msg("Pool members retrieved")
	return vmids
}

// hasTag checks if a VM config contains a specific tag (same as original)
func (h *SearchOptimizedHandler) hasTag(cfg map[string]interface{}, targetTag string) bool {
	tagsStr, ok := cfg["tags"].(string)
	if !ok || tagsStr == "" {
		return false
	}

	targetTag = strings.ToLower(strings.TrimSpace(targetTag))

	// Proxmox can use either semicolon or comma as delimiter
	// Try both separators
	var tags []string
	if strings.Contains(tagsStr, ";") {
		tags = strings.Split(tagsStr, ";")
	} else {
		tags = strings.Split(tagsStr, ",")
	}

	for _, tag := range tags {
		if strings.ToLower(strings.TrimSpace(tag)) == targetTag {
			return true
		}
	}

	return false
}
