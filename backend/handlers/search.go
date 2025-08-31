package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"pvmss/i18n"
	"pvmss/logger"
	"pvmss/proxmox"
	"pvmss/state"

	"github.com/julienschmidt/httprouter"
)

// SearchHandler handles search requests
type SearchHandler struct {
	stateManager state.StateManager
}

// NewSearchHandler creates a new search handler
func NewSearchHandler(sm state.StateManager) *SearchHandler {
	return &SearchHandler{stateManager: sm}
}

// SearchPageHandler handles GET and POST for the search page
func (h *SearchHandler) SearchPageHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	// Create a logger for this request
	log := logger.Get().With().
		Str("handler", "SearchPageHandler").
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("remote_addr", r.RemoteAddr).
		Logger()

	log.Debug().Msg("Handling search request")

	data := make(map[string]interface{})

	// For GET, simply display the search form
	if r.Method == http.MethodGet {
		log.Debug().Msg("Rendering search form")
		// If Proxmox is offline, display a warning but still render the page
		if h.stateManager.GetProxmoxClient() == nil {
			localizer := i18n.GetLocalizer(r)
			data["Warning"] = i18n.Localize(localizer, "Proxmox.NotConnected")
		}
		i18n.LocalizePage(w, r, data)
		data["Title"] = data["Search.Title"]
		renderTemplateInternal(w, r, "search", data)
		log.Info().Msg("Search form rendered successfully")
		return
	}

	// For POST, perform the search
	if r.Method == http.MethodPost {
		// Read and validate search parameters
		vmid := strings.TrimSpace(r.FormValue("vmid"))
		name := strings.TrimSpace(r.FormValue("name"))

		// Preserve submitted values
		data["FormData"] = map[string]string{
			"vmid": vmid,
			"name": name,
		}

		log.Info().
			Str("vmid", vmid).
			Str("name", name).
			Msg("New VM search")

		// Validate inputs
		if vmid == "" && name == "" {
			log.Warn().Msg("No search criteria provided")
			localizer := i18n.GetLocalizer(r)
			data["Error"] = i18n.Localize(localizer, "Search.Validation.MissingCriteria")
			i18n.LocalizePage(w, r, data)
			data["Title"] = data["Search.Title"]
			renderTemplateInternal(w, r, "search", data)
			return
		}

		// Build the query string for display
		var queryParts []string
		if vmid != "" {
			queryParts = append(queryParts, "VMID: "+vmid)
		}
		if name != "" {
			queryParts = append(queryParts, "Name: "+name)
		}
		queryString := strings.Join(queryParts, ", ")
		data["Query"] = queryString

		log.Debug().
			Str("query", queryString).
			Msg("Search criteria formatted")

		// Retrieve Proxmox client from state manager
		client := h.stateManager.GetProxmoxClient()
		if client == nil {
			localizer := i18n.GetLocalizer(r)
			log.Warn().Msg("Proxmox client unavailable; rendering offline-friendly search page")
			data["Error"] = i18n.Localize(localizer, "Proxmox.NotConnected") + ". " + i18n.Localize(localizer, "Proxmox.CheckConnection")
			i18n.LocalizePage(w, r, data)
			data["Title"] = data["Search.Results"]
			renderTemplateInternal(w, r, "search", data)
			return
		}

		log.Debug().Msg("Proxmox client retrieved successfully")

		// Create a context with timeout for the API request
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		log.Debug().Msg("Starting VM search")

		// Retrieve VMs from Proxmox
		vms, err := searchVMs(ctx, client, vmid, name)
		if err != nil {
			log.Error().
				Err(err).
				Msg("VM search failed")

			data["Error"] = fmt.Sprintf("Failed to search for VMs: %v", err)
			i18n.LocalizePage(w, r, data)
			data["Title"] = data["Search.Results"]
			renderTemplateInternal(w, r, "search", data)
			return
		}

		log.Info().
			Int("results_count", len(vms)).
			Msg("VM search completed successfully")

		// Add results to the data map
		data["Results"] = vms
		if len(vms) == 0 {
			log.Debug().Msg("No results found for search")
			data["NoResults"] = true
		} else {
			log.Debug().
				Int("vms_found", len(vms)).
				Msg("VMs found successfully")
		}

		i18n.LocalizePage(w, r, data)
		data["Title"] = data["Search.Results"]

		log.Debug().Msg("Rendering results page")
		renderTemplateInternal(w, r, "search", data)
		log.Info().Msg("Search results rendered successfully")
		return
	}

	// HTTP method not allowed
	log.Warn().
		Str("method", r.Method).
		Msg("HTTP method not allowed for search route")

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// searchVMs searches for VMs based on the provided criteria
func searchVMs(ctx context.Context, clientInterface proxmox.ClientInterface, vmidStr, name string) ([]map[string]interface{}, error) {
	log := logger.Get().With().
		Str("handler", "searchVMs").
		Str("vmid", vmidStr).
		Str("name", name).
		Logger()

	// Prepare search criteria
	lowerNameQuery := strings.ToLower(strings.TrimSpace(name))
	var vmid int
	if vmidStr != "" {
		var err error
		vmid, err = strconv.Atoi(vmidStr)
		if err != nil {
			errMsg := "Invalid VM ID"
			log.Error().
				Err(err).
				Str("vmid_input", vmidStr).
				Msg(errMsg)
			return nil, fmt.Errorf("%s: %v", errMsg, err)
		}
		log.Debug().
			Int("vmid_parsed", vmid).
			Msg("VM ID parsed successfully")
	}

	// Retrieve all VMs
	allVMs, err := proxmox.GetVMsWithContext(ctx, clientInterface)
	if err != nil {
		return nil, fmt.Errorf("failed to get VMs: %w", err)
	}

	// If no criteria are provided, return up to 20 VMs
	if vmidStr == "" && lowerNameQuery == "" {
		results := make([]map[string]interface{}, 0, min(20, len(allVMs)))
		for i, vm := range allVMs {
			if i >= 20 {
				break
			}
			// Determine status: use list status, and if empty try status/current
			status := vm.Status
			if status == "" {
				if cur, err := proxmox.GetVMCurrentWithContext(ctx, clientInterface, vm.Node, vm.VMID); err == nil && cur != nil {
					if cur.Status != "" {
						status = cur.Status
					} else if cur.QMPStatus != "" {
						status = cur.QMPStatus
					}
				}
			}
			if status == "" && vm.Uptime == 0 {
				status = "stopped"
			}
			results = append(results, map[string]interface{}{
				"vmid":   vm.VMID,
				"name":   vm.Name,
				"node":   vm.Node,
				"status": strings.ToLower(status),
			})
		}
		return results, nil
	}

	// Filter VMs according to criteria
	var results []map[string]interface{}

	for _, vm := range allVMs {
		// VMID check (if provided)
		if vmid > 0 && vm.VMID != vmid {
			continue
		}

		// Name contains check (if provided)
		if lowerNameQuery != "" && !strings.Contains(strings.ToLower(vm.Name), lowerNameQuery) {
			continue
		}

		// Derive status: prefer list status; if empty, try status/current
		status := vm.Status
		if status == "" {
			if cur, err := proxmox.GetVMCurrentWithContext(ctx, clientInterface, vm.Node, vm.VMID); err == nil && cur != nil {
				if cur.Status != "" {
					status = cur.Status
				} else if cur.QMPStatus != "" {
					status = cur.QMPStatus
				}
			}
		}
		results = append(results, map[string]interface{}{
			"vmid":   vm.VMID,
			"name":   vm.Name,
			"node":   vm.Node,
			"status": strings.ToLower(status),
		})

		// If VMID is specified, it's unique; return early after first match
		if vmid > 0 {
			break
		}
	}

	log.Info().
		Int("matching_vms", len(results)).
		Int("total_vms_searched", len(allVMs)).
		Msg("VM filtering completed successfully")

	return results, nil
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

	router.GET("/search", h.SearchPageHandler)
	router.POST("/search", h.SearchPageHandler)

	log.Info().
		Strs("routes", []string{"GET /search", "POST /search"}).
		Msg("Search routes registered successfully")
}

// SearchHandlerFunc is a wrapper function for compatibility with existing code
func SearchHandlerFunc(w http.ResponseWriter, r *http.Request) {
	log := logger.Get().With().
		Str("function", "SearchHandlerFunc").
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Logger()

	log.Debug().Msg("Calling search handler via wrapper function")

	h := &SearchHandler{stateManager: getStateManager(r)}
	h.SearchPageHandler(w, r, nil)

	log.Debug().Msg("Search handler processing finished")
}
