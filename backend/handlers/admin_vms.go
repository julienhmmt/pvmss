package handlers

import (
	"context"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"

	"pvmss/constants"
	"pvmss/i18n"
	"pvmss/logger"
	"pvmss/proxmox"
	"pvmss/state"

	"github.com/julienschmidt/httprouter"
)

// AdminVMsHandler handles admin VM listing operations
type AdminVMsHandler struct {
	stateManager state.StateManager
}

// NewAdminVMsHandler creates a new instance of AdminVMsHandler
func NewAdminVMsHandler(sm state.StateManager) *AdminVMsHandler {
	return &AdminVMsHandler{stateManager: sm}
}

// AdminVMInfo represents VM information for admin display
type AdminVMInfo struct {
	VMID   int
	Name   string
	Node   string
	Status string
	Tags   string
}

// VMsPageHandler handles the admin VMs page with pagination support
func (h *AdminVMsHandler) VMsPageHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("AdminVMsPageHandler", r)

	// Parse pagination parameters
	page := 1
	limit := 25

	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	offset := (page - 1) * limit

	// Proxmox connection status
	proxmoxConnected, proxmoxMsg := h.stateManager.GetProxmoxStatus()
	client := h.stateManager.GetProxmoxClient()
	offlineMode := h.stateManager.IsOfflineMode()

	var vms []AdminVMInfo
	var totalVMs int
	var errMsg string

	if proxmoxConnected && client != nil {
		ctx, cancel := context.WithTimeout(r.Context(), constants.ShortContextTimeout)
		defer cancel()

		// Get all VMs with pvmss tag first to get total count
		allVMs, retrievalErrKey := h.getVMsWithPVMSSTag(ctx)
		if retrievalErrKey == "" {
			totalVMs = len(allVMs)

			// Apply pagination
			start := offset
			end := offset + limit

			if start < len(allVMs) {
				if end > len(allVMs) {
					end = len(allVMs)
				}
				vms = allVMs[start:end]
			}
		} else {
			// errMsg here is an i18n key; localize it for display
			localized := i18n.Localize(i18n.GetLocalizerFromRequest(r), retrievalErrKey)
			log.Warn().Str("error", localized).Msg("Failed to retrieve VMs")
			errMsg = localized
		}
	} else {
		if offlineMode {
			log.Info().Msg("Offline mode enabled; skipping Proxmox VM retrieval")
		} else {
			// Localize generic connection unavailable if no specific proxmox message
			errMsg = i18n.Localize(i18n.GetLocalizerFromRequest(r), "Admin.VMs.Error.ConnectionUnavailable")
			if proxmoxMsg != "" {
				errMsg = proxmoxMsg
			}
			log.Warn().Msg("Proxmox client is not initialized")
		}
	}

	// Build success message from query params
	successMsg := ""
	if r.URL.Query().Get("success") == "1" {
		successMsg = i18n.Localize(i18n.GetLocalizerFromRequest(r), "Admin.VMs.Success.OperationCompleted")
	}

	// Calculate pagination info
	totalPages := (totalVMs + limit - 1) / limit
	hasNextPage := page < totalPages
	hasPrevPage := page > 1

	// Generate page numbers for pagination (show max 5 pages around current page)
	var paginationPages []int
	startPage := page - 2
	if startPage < 1 {
		startPage = 1
	}
	endPage := startPage + 4
	if endPage > totalPages {
		endPage = totalPages
	}
	for i := startPage; i <= endPage; i++ {
		paginationPages = append(paginationPages, i)
	}

	// Calculate pagination display info
	from := offset + 1
	to := offset + len(vms)

	opts := []TemplateOption{
		WithAdminActive("vms"),
		WithAuth(r),
		WithProxmoxStatus(h.stateManager),
		WithMessages(r),
		WithData("TitleKey", "Admin.VMs.Title"),
		WithData("VMs", vms),
		WithData("TotalVMs", totalVMs),
		WithData("CurrentPage", page),
		WithData("Limit", limit),
		WithData("TotalPages", totalPages),
		WithData("HasNextPage", hasNextPage),
		WithData("HasPrevPage", hasPrevPage),
		WithData("NextPage", page+1),
		WithData("PrevPage", page-1),
		WithData("PaginationPages", paginationPages),
		WithData("PaginationInfo", map[string]int{
			"From": from,
			"To":   to,
		}),
		WithData("OfflineMode", offlineMode),
	}

	if successMsg != "" {
		opts = append(opts, WithSuccess(successMsg))
	}

	if errMsg != "" {
		opts = append(opts, WithError(errMsg))
	}

	data := NewTemplateDataWithOptions("", opts...).ToMap()
	renderTemplateInternal(w, r, "admin_vms", data)
}

// getVMsWithPVMSSTag retrieves all VMs that have the pvmss tag using resty
func (h *AdminVMsHandler) getVMsWithPVMSSTag(ctx context.Context) ([]AdminVMInfo, string) {
	log := logger.Get().With().
		Str("function", "getVMsWithPVMSSTag").
		Logger()

	// Create resty client
	restyClient, err := getDefaultRestyClient()
	if err != nil {
		log.Error().Err(err).Msg("Failed to create resty client")
		return nil, "Admin.VMs.Error.CreateAPIClient"
	}

	// Get all VMs from Proxmox using resty
	allVMs, err := proxmox.GetVMsResty(ctx, restyClient)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get VMs (resty)")
		return nil, "Admin.VMs.Error.RetrieveVMsFromProxmox"
	}

	log.Info().Int("total_vms", len(allVMs)).Msg("Retrieved all VMs (resty)")

	// Filter VMs with pvmss tag using concurrent config fetch
	results := make([]AdminVMInfo, 0, len(allVMs))
	var mu sync.Mutex
	// Limit parallelism to avoid overwhelming API
	sem := make(chan struct{}, 8)
	g, gctx := errgroup.WithContext(ctx)

	for i := range allVMs {
		vm := allVMs[i] // capture
		g.Go(func() error {
			// Acquire semaphore slot
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-gctx.Done():
				return gctx.Err()
			}

			cfg, err := proxmox.GetVMConfigResty(gctx, restyClient, vm.Node, vm.VMID)
			if err != nil {
				log.Debug().Err(err).Int("vmid", vm.VMID).Msg("Failed to get VM config, skipping")
				return nil
			}

			if !h.hasTag(cfg, "pvmss") {
				return nil
			}

			status := vm.Status
			if status == "" {
				status = "unknown"
			}

			tags := ""
			if tagsValue, ok := cfg["tags"].(string); ok {
				tags = tagsValue
			}

			info := AdminVMInfo{
				VMID:   vm.VMID,
				Name:   vm.Name,
				Node:   vm.Node,
				Status: strings.ToLower(status),
				Tags:   tags,
			}

			mu.Lock()
			results = append(results, info)
			mu.Unlock()

			log.Debug().
				Int("vmid", vm.VMID).
				Str("name", vm.Name).
				Str("node", vm.Node).
				Msg("VM with pvmss tag found")
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		log.Warn().Err(err).Msg("Concurrent VM config fetch encountered errors")
	}

	// Sort by VMID
	sort.Slice(results, func(i, j int) bool {
		return results[i].VMID < results[j].VMID
	})

	log.Info().
		Int("total_found", len(results)).
		Int("total_checked", len(allVMs)).
		Msg("Completed filtering VMs with pvmss tag (concurrent)")

	return results, ""
}

// hasTag checks if a VM config contains a specific tag
func (h *AdminVMsHandler) hasTag(cfg map[string]interface{}, targetTag string) bool {
	tagsStr, ok := cfg["tags"].(string)
	if !ok || tagsStr == "" {
		return false
	}

	targetTag = strings.ToLower(strings.TrimSpace(targetTag))

	// Proxmox can use either semicolon or comma as delimiter
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

// RegisterRoutes registers the routes for admin VM listing
func (h *AdminVMsHandler) RegisterRoutes(router *httprouter.Router) {
	log := logger.Get().With().
		Str("component", "AdminVMsHandler").
		Str("function", "RegisterRoutes").
		Logger()

	if router == nil {
		log.Error().Msg("Router is nil, cannot register admin VMs routes")
		return
	}

	log.Debug().Msg("Registering admin VMs routes")

	// Register admin VMs page
	router.GET("/admin/vms", HandlerFuncToHTTPrHandle(RequireAdminAuth(func(w http.ResponseWriter, r *http.Request) {
		h.VMsPageHandler(w, r, httprouter.ParamsFromContext(r.Context()))
	})))

	log.Info().
		Strs("routes", []string{"GET /admin/vms"}).
		Msg("Admin VMs routes registered successfully")
}
