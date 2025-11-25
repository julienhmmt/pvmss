package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
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

	log.Debug().
		Str("component", "search").
		Str("operation", "register_routes").
		Str("reason", "route_registration").
		Msg("Registering optimized search routes")

	router.GET("/search", RequireAuthHandle(h.SearchPageHandler))
	router.POST("/search", SecureFormHandler("Search",
		RequireAuthHandle(h.SearchPageHandler),
	))

	log.Info().
		Strs("routes", []string{"GET /search", "POST /search"}).
		Msg("Optimized search routes registered successfully")
}

// RegisterAJAXRoutes registers AJAX search routes
func (h *SearchOptimizedHandler) RegisterAJAXRoutes(router *httprouter.Router) {
	log := logger.Get().With().
		Str("component", "SearchOptimizedHandler").
		Str("function", "RegisterAJAXRoutes").
		Logger()

	if router == nil {
		log.Error().Msg("Router is nil, cannot register AJAX search routes")
		return
	}

	log.Debug().
		Str("component", "search").
		Str("operation", "register_ajax_routes").
		Str("reason", "ajax_route_registration").
		Msg("Registering AJAX search routes")

	router.GET("/api/search/vms", RequireAuthHandle(h.SearchAPIHandler))

	log.Info().
		Strs("routes", []string{"GET /api/search/vms"}).
		Msg("AJAX search routes registered successfully")
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
		tagQuery := strings.TrimSpace(r.FormValue("tag"))

		// Parse tag query for multiple tags (space-separated)
		var tagQueries []string
		if tagQuery != "" {
			// Split by spaces and filter out empty strings
			tagParts := strings.Fields(strings.TrimSpace(tagQuery))
			for _, tag := range tagParts {
				if tag := strings.TrimSpace(tag); tag != "" {
					tagQueries = append(tagQueries, strings.ToLower(tag))
				}
			}
		}

		log.Info().
			Str("vmid_query", vmidQuery).
			Str("name_query", nameQuery).
			Strs("tag_queries", tagQueries).
			Msg("Processing optimized search query")

		// Build query display string
		var queryParts []string
		if vmidQuery != "" {
			queryParts = append(queryParts, "VMID: "+vmidQuery)
		}
		if nameQuery != "" {
			queryParts = append(queryParts, "Name: "+nameQuery)
		}
		if len(tagQueries) > 0 {
			tagDisplay := strings.Join(tagQueries, ", ")
			queryParts = append(queryParts, "Tags: "+tagDisplay)
		}
		queryDisplay := strings.Join(queryParts, ", ")
		if queryDisplay == "" {
			queryDisplay = "All VMs"
		}

		data["Query"] = queryDisplay
		data["FormData"] = map[string]string{
			"vmid": vmidQuery,
			"name": nameQuery,
			"tag":  tagQuery,
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
		results, err := h.searchVMsOptimized(ctx, client, vmidQuery, nameQuery, tagQueries, username, isAdmin)
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
// TODO Telmate migration: this search helper still calls GetVMConfigWithContext for tag and pvmss checks; switch to Resty-based VM config helpers and remove the Telmate ClientInterface.
func (h *SearchOptimizedHandler) searchVMsOptimized(ctx context.Context, client proxmox.ClientInterface, vmidQuery, nameQuery string, tagQueries []string, username string, isAdmin bool) ([]map[string]interface{}, error) {
	log := logger.Get().With().
		Str("function", "searchVMsOptimized").
		Str("vmid_query", vmidQuery).
		Str("name_query", nameQuery).
		Strs("tag_queries", tagQueries).
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
		if vmidQuery != "" || nameQuery != "" || len(tagQueries) > 0 {
			vmidStr := strconv.Itoa(vm.VMID)
			vmName := strings.ToLower(vm.Name)

			matchesVMID := lowerVMIDQuery != "" && strings.Contains(vmidStr, lowerVMIDQuery)
			matchesName := lowerNameQuery != "" && strings.Contains(vmName, lowerNameQuery)

			// For tag matching, we need to get the config first
			matchesTags := false
			if len(tagQueries) > 0 {
				// Get config to check tags
				cfg, err := proxmox.GetVMConfigWithContext(ctx, client, vm.Node, vm.VMID)
				if err != nil {
					log.Debug().
						Err(err).
						Int("vmid", vm.VMID).
						Str("component", "search").
						Str("operation", "tag_filtering").
						Str("reason", "vm_config_failed").
						Msg("Failed to get VM config for tag filtering, skipping")
					continue
				}

				// Must have 'pvmss' tag AND all searched tags
				hasPVMSS := h.hasTag(cfg, "pvmss")
				if !hasPVMSS {
					continue // Skip if no pvmss tag
				}

				// Check if VM has ALL the searched tags
				hasAllSearchedTags := true
				for _, searchedTag := range tagQueries {
					if !h.hasTag(cfg, searchedTag) {
						hasAllSearchedTags = false
						break
					}
				}
				matchesTags = hasAllSearchedTags

				// If tag matching fails, skip this VM
				if !matchesTags {
					continue
				}
			} else {
				// If no tag query, still require 'pvmss' tag
				cfg, err := proxmox.GetVMConfigWithContext(ctx, client, vm.Node, vm.VMID)
				if err != nil {
					log.Debug().
						Err(err).
						Int("vmid", vm.VMID).
						Str("component", "search").
						Str("operation", "pvmss_tag_check").
						Str("reason", "vm_config_failed").
						Msg("Failed to get VM config for pvmss tag check, skipping")
					continue
				}

				if !h.hasTag(cfg, "pvmss") {
					log.Debug().
						Int("vmid", vm.VMID).
						Str("name", vm.Name).
						Str("component", "search").
						Str("operation", "pvmss_tag_check").
						Str("reason", "missing_pvmss_tag").
						Msg("VM does not have pvmss tag, skipping")
					continue
				}
			}

			// If both queries provided, match either VMID or name (in addition to tag requirements)
			// If only one query provided, must match that one (in addition to tag requirements)
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
		} else {
			// No search criteria provided, still require 'pvmss' tag
			cfg, err := proxmox.GetVMConfigWithContext(ctx, client, vm.Node, vm.VMID)
			if err != nil {
				log.Debug().
					Err(err).
					Int("vmid", vm.VMID).
					Str("component", "search").
					Str("operation", "pvmss_tag_check").
					Str("reason", "vm_config_failed").
					Msg("Failed to get VM config for pvmss tag check, skipping")
				continue
			}

			if !h.hasTag(cfg, "pvmss") {
				log.Debug().
					Int("vmid", vm.VMID).
					Str("name", vm.Name).
					Msg("VM does not have pvmss tag, skipping")
				continue
			}
		}

		// VM passed initial filters, add to filtered list
		filteredVMs = append(filteredVMs, vmInfo)
	}

	log.Info().
		Int("filtered_vms", len(filteredVMs)).
		Int("original_vms", len(allVMs)).
		Msg("VMs filtered before config check")

	// BATCH: Get configs only for filtered VMs that haven't been checked yet
	// (we already checked tags for VMs with tag queries, but need configs for others)
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
				log.Debug().
					Err(err).
					Str("vmid", vmInfo.VMID).
					Str("component", "search").
					Str("operation", "vmid_validation").
					Str("reason", "invalid_vmid").
					Msg("Invalid VMID, skipping")
				return
			}

			// Get config (we may already have it from tag filtering, but get it again for consistency)
			cfg, err := proxmox.GetVMConfigWithContext(ctx, client, vmInfo.Node, vmidInt)
			if err != nil {
				log.Debug().
					Err(err).
					Int("vmid", vmidInt).
					Str("component", "search").
					Str("operation", "fetch_vm_config").
					Str("reason", "config_fetch_failed").
					Msg("Failed to get VM config, skipping")
				return
			}

			// Double-check for pvmss tag (safety check)
			if !h.hasTag(cfg, "pvmss") {
				log.Debug().
					Int("vmid", vmidInt).
					Str("name", vmInfo.Name).
					Str("component", "search").
					Str("operation", "final_pvmss_check").
					Str("reason", "missing_pvmss_tag").
					Msg("VM does not have pvmss tag in final check, skipping")
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

		// Extract tags from config
		tags := []string{}
		if tagsStr, ok := cfg["tags"].(string); ok && tagsStr != "" {
			// Proxmox can use either semicolon or comma as delimiter
			var tagList []string
			if strings.Contains(tagsStr, ";") {
				tagList = strings.Split(tagsStr, ";")
			} else {
				tagList = strings.Split(tagsStr, ",")
			}
			for _, tag := range tagList {
				tag = strings.TrimSpace(tag)
				if tag != "" {
					tags = append(tags, tag)
				}
			}
		}

		status := vm.Status
		if status == "" {
			status = "unknown"
		}

		results = append(results, map[string]interface{}{
			"vmid":        vmidInt,
			"name":        vm.Name,
			"description": description,
			"node":        vm.Node,
			"tags":        tags,
			"status":      strings.ToLower(status),
		})
	}

	// Sort results by VMID ascending before returning
	sort.Slice(results, func(i, j int) bool {
		vi, _ := results[i]["vmid"].(int)
		vj, _ := results[j]["vmid"].(int)
		return vi < vj
	})

	// Apply limit after sorting for deterministic ordering
	if len(results) > 50 {
		results = results[:50]
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
		log.Warn().
			Err(err).
			Str("component", "search").
			Str("operation", "fetch_pool_members").
			Str("reason", "pool_fetch_failed").
			Msg("Failed to fetch pool members")
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

	log.Debug().
		Int("vm_count", len(vmids)).
		Str("component", "search").
		Str("operation", "fetch_pool_members").
		Str("reason", "pool_retrieved").
		Msg("Pool members retrieved")
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

// SearchAPIHandler handles AJAX search requests returning JSON
func (h *SearchOptimizedHandler) SearchAPIHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("SearchAPIHandler", r)

	// Set JSON content type
	w.Header().Set("Content-Type", "application/json")

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

	// Parse query parameters
	vmidQuery := strings.TrimSpace(r.URL.Query().Get("vmid"))
	nameQuery := strings.TrimSpace(r.URL.Query().Get("name"))
	tagsQuery := strings.TrimSpace(r.URL.Query().Get("tags"))
	limitStr := strings.TrimSpace(r.URL.Query().Get("limit"))

	// Parse limit (default 50, max 200)
	limit := 50
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= 200 {
			limit = parsedLimit
		}
	}

	log.Info().
		Str("username", username).
		Bool("is_admin", isAdmin).
		Str("vmid_query", vmidQuery).
		Str("name_query", nameQuery).
		Str("tags_query", tagsQuery).
		Int("limit", limit).
		Msg("AJAX search request")

	// Get Proxmox client
	client := h.stateManager.GetProxmoxClient()
	if client == nil {
		log.Error().Msg("Proxmox client not available")
		w.WriteHeader(http.StatusServiceUnavailable)
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Proxmox connection not available",
		}); err != nil {
			log.Error().Err(err).Msg("Failed to encode Proxmox unavailable response")
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
		return
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Parse tags (split by spaces and clean)
	var tagsFilter []string
	if tagsQuery != "" {
		rawTags := strings.Fields(tagsQuery)
		for _, tag := range rawTags {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				tagsFilter = append(tagsFilter, tag)
			}
		}
	}

	// Perform search
	results, err := h.searchVMsAJAX(ctx, client, vmidQuery, nameQuery, tagsFilter, username, isAdmin, limit)
	if err != nil {
		log.Error().Err(err).Msg("AJAX search failed")
		w.WriteHeader(http.StatusInternalServerError)
		if encodeErr := json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Search failed: %v", err),
		}); encodeErr != nil {
			log.Error().Err(encodeErr).Msg("Failed to encode search error response")
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
		return
	}

	log.Info().
		Int("results_count", len(results)).
		Msg("AJAX search completed successfully")

	// Return JSON response
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"results": results,
		"count":   len(results),
	}); err != nil {
		log.Error().Err(err).Msg("Failed to encode search success response")
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// searchVMsAJAX performs VM search for AJAX API with advanced filtering
// TODO Telmate migration: this AJAX search helper still calls GetVMConfigWithContext for tag and pvmss checks; switch to Resty-based VM config helpers and remove the Telmate ClientInterface.
func (h *SearchOptimizedHandler) searchVMsAJAX(ctx context.Context, client proxmox.ClientInterface, vmidQuery, nameQuery string, tagsFilter []string, username string, isAdmin bool, limit int) ([]map[string]interface{}, error) {
	log := logger.Get().With().
		Str("function", "searchVMsAJAX").
		Str("vmid_query", vmidQuery).
		Str("name_query", nameQuery).
		Strs("tags_filter", tagsFilter).
		Str("username", username).
		Bool("is_admin", isAdmin).
		Int("limit", limit).
		Logger()

	// Get all VMs from Proxmox using optimized approach
	restyClient, err := getDefaultRestyClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create resty client: %w", err)
	}

	allVMs, err := proxmox.GetVMsResty(ctx, restyClient)
	if err != nil {
		return nil, fmt.Errorf("failed to get VMs: %w", err)
	}

	log.Info().Int("total_vms", len(allVMs)).Msg("Retrieved all VMs for AJAX search")

	// For non-admin users, get their pool VMs
	var userPoolVMIDs map[int]bool
	if !isAdmin && username != "" {
		poolName := "pvmss_" + username
		userPoolVMIDs = h.getPoolVMIDs(ctx, client, poolName)
		log.Info().
			Str("pool", poolName).
			Int("pool_vm_count", len(userPoolVMIDs)).
			Msg("Retrieved user pool VMs for AJAX search")
	}

	// Filter VMs
	results := []map[string]interface{}{}
	lowerVMIDQuery := strings.ToLower(vmidQuery)
	lowerNameQuery := strings.ToLower(nameQuery)
	lowerTagsFilter := make([]string, len(tagsFilter))
	for i, tag := range tagsFilter {
		lowerTagsFilter[i] = strings.ToLower(tag)
	}

	for _, vm := range allVMs {
		// Check 1: Pool membership for non-admin users
		if !isAdmin && userPoolVMIDs != nil {
			if !userPoolVMIDs[vm.VMID] {
				continue // VM not in user's pool
			}
		}

		// Check 2: Get VM config and check for "pvmss" tag
		cfg, err := proxmox.GetVMConfigWithContext(ctx, client, vm.Node, vm.VMID)
		if err != nil {
			log.Debug().
				Err(err).
				Int("vmid", vm.VMID).
				Str("component", "search").
				Str("operation", "ajax_search").
				Str("reason", "vm_config_failed").
				Msg("Failed to get VM config for AJAX search, skipping")
			continue
		}

		// Check for pvmss tag
		if !h.hasTag(cfg, "pvmss") {
			continue
		}

		// Check 3: VMID or Name search (if provided)
		if vmidQuery != "" || nameQuery != "" || len(tagsFilter) > 0 {
			vmidStr := strconv.Itoa(vm.VMID)
			vmName := strings.ToLower(vm.Name)

			matchesVMID := lowerVMIDQuery != "" && strings.Contains(vmidStr, lowerVMIDQuery)
			matchesName := lowerNameQuery != "" && strings.Contains(vmName, lowerNameQuery)

			// For tags: VM must have ALL searched tags
			matchesTags := false
			if len(tagsFilter) > 0 {
				// Get config to check tags
				cfg, err := proxmox.GetVMConfigWithContext(ctx, client, vm.Node, vm.VMID)
				if err != nil {
					log.Debug().
						Err(err).
						Int("vmid", vm.VMID).
						Str("component", "search").
						Str("operation", "tag_filtering").
						Str("reason", "vm_config_failed").
						Msg("Failed to get VM config for tag filtering, skipping")
					continue
				}

				// Must have 'pvmss' tag AND all searched tags
				hasPVMSS := h.hasTag(cfg, "pvmss")
				if !hasPVMSS {
					continue // Skip if no pvmss tag
				}

				// Check if VM has ALL the searched tags
				hasAllSearchedTags := true
				for _, searchedTag := range tagsFilter {
					if !h.hasTag(cfg, searchedTag) {
						hasAllSearchedTags = false
						break
					}
				}
				matchesTags = hasAllSearchedTags

				// If tag matching fails, skip this VM
				if !matchesTags {
					continue
				}
			}

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

		// VM passed all filters, add to results
		description := ""
		if desc, ok := cfg["description"].(string); ok {
			description = desc
		}

		// Extract additional metadata for AJAX response
		tags := ""
		if tagsStr, ok := cfg["tags"].(string); ok {
			tags = tagsStr
		}

		// Get status
		vmStatus := strings.ToLower(vm.Status)
		if vmStatus == "" {
			vmStatus = "unknown"
		}

		results = append(results, map[string]interface{}{
			"vmid":        vm.VMID,
			"name":        vm.Name,
			"description": description,
			"node":        vm.Node,
			"status":      vmStatus,
			"tags":        tags,
		})

		// Limit results
		if len(results) >= limit {
			break
		}
	}

	log.Info().
		Int("results_count", len(results)).
		Int("vms_checked", len(allVMs)).
		Msg("AJAX search filtering completed")

	return results, nil
}
