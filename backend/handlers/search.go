package handlers

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"pvmss/i18n"
	"pvmss/logger"
	"pvmss/proxmox"
	"pvmss/security"
	"pvmss/state"

	"github.com/julienschmidt/httprouter"
)

// SearchHandler handles search requests with simplified logic
type SearchHandler struct {
	stateManager state.StateManager
}

// NewSearchHandler creates a new instance of SearchHandler
func NewSearchHandler(sm state.StateManager) *SearchHandler {
	return &SearchHandler{stateManager: sm}
}

// RegisterRoutes registers search routes
func (h *SearchHandler) RegisterRoutes(router *httprouter.Router) {
	log := logger.Get().With().
		Str("component", "SearchHandler").
		Str("function", "RegisterRoutes").
		Logger()

	if router == nil {
		log.Error().Msg("Router is nil, cannot register search routes")
		return
	}

	log.Debug().Msg("Registering search routes")

	router.GET("/search", RequireAuthHandle(h.SearchPageHandler))
	router.POST("/search", SecureFormHandler("Search",
		RequireAuthHandle(h.SearchPageHandler),
	))

	log.Info().
		Strs("routes", []string{"GET /search", "POST /search"}).
		Msg("Search routes registered successfully")
}

// SearchPageHandler handles both GET and POST requests for search page
func (h *SearchHandler) SearchPageHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
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
		Msg("Search request started")

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
			Msg("Processing search query")

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

		// Create context with timeout
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		// Perform search
		results, err := h.searchVMs(ctx, client, vmidQuery, nameQuery, username, isAdmin)
		if err != nil {
			log.Error().Err(err).Msg("Search failed")
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
			Msg("Search completed successfully")
	}

	renderTemplateInternal(w, r, "search", data)
}

// searchVMs performs the actual VM search with filtering using resty
func (h *SearchHandler) searchVMs(ctx context.Context, client proxmox.ClientInterface, vmidQuery, nameQuery, username string, isAdmin bool) ([]map[string]interface{}, error) {
	log := logger.Get().With().
		Str("function", "searchVMs").
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

	// Filter VMs
	results := []map[string]interface{}{}
	lowerVMIDQuery := strings.ToLower(vmidQuery)
	lowerNameQuery := strings.ToLower(nameQuery)

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
			log.Debug().Err(err).Int("vmid", vm.VMID).Msg("Failed to get VM config, skipping")
			continue
		}

		// Check for pvmss tag
		if !h.hasTag(cfg, "pvmss") {
			log.Info().
				Int("vmid", vm.VMID).
				Str("name", vm.Name).
				Interface("tags", cfg["tags"]).
				Msg("SEARCH V2: VM does not have pvmss tag, skipping")
			continue
		}

		// Check 3: Match search criteria (if provided)
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

		// VM passed all filters, add to results
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

		log.Info().
			Int("vmid", vm.VMID).
			Str("name", vm.Name).
			Msg("SEARCH V2: VM matched all criteria")

		// Limit results to 50
		if len(results) >= 50 {
			break
		}
	}

	log.Info().
		Int("results_count", len(results)).
		Int("vms_checked", len(allVMs)).
		Msg("Search filtering completed")

	return results, nil
}

// getPoolVMIDs retrieves VM IDs from a Proxmox pool
func (h *SearchHandler) getPoolVMIDs(ctx context.Context, client proxmox.ClientInterface, poolName string) map[int]bool {
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

	if err := client.GetJSON(ctx, "/pools/"+url.PathEscape(poolName), &poolResp); err != nil {
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

// hasTag checks if a VM config contains a specific tag
func (h *SearchHandler) hasTag(cfg map[string]interface{}, targetTag string) bool {
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
