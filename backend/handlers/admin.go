package handlers

import (
	"context"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"

	"pvmss/components"
	"pvmss/constants"
	"pvmss/i18n"
	"pvmss/proxmox"
	"pvmss/state"
	"pvmss/utils"
)

// AdminOptimizedHandler handles administration routes with optimized cluster performance
type AdminOptimizedHandler struct {
	stateManager state.StateManager
}

// NewAdminOptimizedHandler creates a new instance of AdminOptimizedHandler
func NewAdminOptimizedHandler(sm state.StateManager) *AdminOptimizedHandler {
	return &AdminOptimizedHandler{stateManager: sm}
}

// RegisterRoutes registers admin routes
func (h *AdminOptimizedHandler) RegisterRoutes(router *httprouter.Router) {
	log := CreateHandlerLogger("AdminOptimizedHandler", nil)

	if router == nil {
		log.Error().Msg("Router is nil, cannot register admin routes")
		return
	}

	log.Debug().
		Str("component", "admin").
		Str("operation", "register_routes").
		Msg("Registering optimized admin routes")

	// Admin main page
	router.GET("/admin", HandlerFuncToHTTPrHandle(RequireAdminAuth(func(w http.ResponseWriter, r *http.Request) {
		h.AdminPageHandler(w, r, httprouter.ParamsFromContext(r.Context()))
	})))

	// Admin nodes page (optimized)
	router.GET("/admin/nodes", RequireAuthHandle(h.NodesPageHandlerOptimized))

	// Admin application info page
	router.GET("/admin/appinfo", HandlerFuncToHTTPrHandle(RequireAdminAuth(func(w http.ResponseWriter, r *http.Request) {
		h.AppInfoPageHandler(w, r, httprouter.ParamsFromContext(r.Context()))
	})))

	// Admin Proxmox ticket test page
	router.GET("/admin/ticket-test", HandlerFuncToHTTPrHandle(RequireAdminAuth(func(w http.ResponseWriter, r *http.Request) {
		h.ProxmoxTicketTestPageHandler(w, r, httprouter.ParamsFromContext(r.Context()))
	})))

	// Admin Proxmox ticket test form
	router.POST("/admin/ticket-test", SecureFormHandler("ProxmoxTicketTest",
		HandlerFuncToHTTPrHandle(RequireAdminAuth(func(w http.ResponseWriter, r *http.Request) {
			h.ProxmoxTicketTestFormHandler(w, r, httprouter.ParamsFromContext(r.Context()))
		})),
	))

	log.Info().
		Str("route", "GET /admin/nodes").
		Msg("Optimized admin route registered successfully")
}

// NodesPageHandlerOptimized renders the Nodes admin page with optimizations
func (h *AdminOptimizedHandler) NodesPageHandlerOptimized(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("NodesPageHandlerOptimized", r)

	refreshNode := strings.TrimSpace(r.URL.Query().Get("refreshNode"))

	// Proxmox connection status from background monitor
	proxmoxConnected, _ := h.stateManager.GetProxmoxStatus()
	var nodeDetails []*proxmox.NodeDetails
	var errMsg string
	nodeDataSource := "live"
	nodeCacheAgeSeconds := 0
	nodeLastUpdate := make(map[string]string)
	const nodeTimeLayout = "2006-01-02 15:04:05"
	nodeLocation := time.Local
	if tz := os.Getenv("TZ"); tz != "" {
		if loc, err := time.LoadLocation(tz); err == nil {
			nodeLocation = loc
		} else {
			log.Warn().
				Str("component", "admin_nodes").
				Str("operation", "resolve_timezone").
				Str("tz", tz).
				Err(err).
				Msg("Failed to load TZ location, falling back to server local time")
		}
	}

	if cachedDetails, cacheTimestamp := h.stateManager.GetNodeCache(); len(cachedDetails) > 0 {
		nodeDetails = cachedDetails
		nodeDataSource = "cache"
		age := int(time.Since(cacheTimestamp).Seconds())
		if age < 0 {
			age = 0
		}
		nodeCacheAgeSeconds = age
		for _, d := range cachedDetails {
			if d == nil || d.Node == "" {
				continue
			}
			nodeLastUpdate[d.Node] = cacheTimestamp.In(nodeLocation).Format(nodeTimeLayout)
		}
		log.Debug().
			Int("node_details_count", len(nodeDetails)).
			Int("cache_age_seconds", nodeCacheAgeSeconds).
			Str("component", "admin").
			Str("operation", "serve_node_cache").
			Msg("Serving node details from cache")
	}

	if len(nodeDetails) == 0 {
		if proxmoxConnected {
			// Create a resty client for this request
			proxmoxURL := os.Getenv("PROXMOX_URL")
			tokenID := os.Getenv("PROXMOX_API_TOKEN_NAME")
			tokenValue := os.Getenv("PROXMOX_API_TOKEN_VALUE")
			insecureSkipVerify := os.Getenv("PROXMOX_VERIFY_SSL") == "false"

			if proxmoxURL != "" && tokenID != "" && tokenValue != "" {
				restyClient, err := proxmox.NewRestyClient(proxmoxURL, tokenID, tokenValue, insecureSkipVerify, constants.ShortContextTimeout)
				if err != nil {
					log.Error().Err(err).Msg("Failed to create resty client")
					errMsg = "Failed to create API client"
				} else {
					// Use optimized context timeout
					ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
					defer cancel()

					log.Info().Msg("Using optimized resty client to fetch nodes")

					// Get node details with optimized batch processing
					nodeDetails, err = h.getNodeDetailsOptimized(ctx, restyClient)
					if err != nil {
						log.Warn().
							Err(err).
							Str("component", "admin_nodes").
							Str("operation", "retrieve_node_details").
							Str("method", "optimized").
							Msg("Unable to retrieve Proxmox node details")
						errMsg = "Failed to retrieve node details"
					} else {
						log.Info().Int("node_details_count", len(nodeDetails)).Msg("Successfully fetched node details with optimization")
						nodeCacheAgeSeconds = 0
						now := time.Now().In(nodeLocation)
						for _, d := range nodeDetails {
							if d == nil || d.Node == "" {
								continue
							}
							nodeLastUpdate[d.Node] = now.Format(nodeTimeLayout)
						}
					}
				}
			} else {
				log.Warn().
					Str("component", "admin_nodes").
					Str("operation", "retrieve_node_details").
					Str("reason", "credentials_not_configured").
					Msg("Proxmox credentials not configured")
				errMsg = "Proxmox credentials missing"
			}
		} else {
			log.Warn().
				Str("component", "admin_nodes").
				Str("operation", "retrieve_node_details").
				Str("reason", "client_offline").
				Str("fallback", "cached_data").
				Msg("Proxmox client offline; using cached data")
			errMsg = "Proxmox connection unavailable"
		}
	} else {
		log.Debug().
			Int("node_details_count", len(nodeDetails)).
			Str("source", nodeDataSource).
			Str("component", "admin").
			Str("operation", "render_node_cache").
			Msg("Rendering node details from cache")
	}

	// Optional per-node refresh: when a refreshNode query parameter is provided,
	// try to re-fetch only that node from Proxmox to update its metrics without
	// waiting for the background cache worker.
	if refreshNode != "" && proxmoxConnected {
		restyClient, err := proxmox.NewRestyClientFromEnv(constants.ShortContextTimeout)
		if err != nil {
			log.Warn().
				Err(err).
				Str("component", "admin_nodes").
				Str("operation", "refresh_single_node").
				Str("node", refreshNode).
				Str("reason", "client_creation_failed").
				Msg("Failed to create resty client for single node refresh")
		} else {
			ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
			defer cancel()

			detail, detailErr := proxmox.GetNodeDetailsResty(ctx, restyClient, refreshNode)
			if detailErr != nil {
				log.Warn().
					Err(detailErr).
					Str("component", "admin_nodes").
					Str("operation", "refresh_single_node").
					Str("node", refreshNode).
					Msg("Failed to refresh single node details; keeping cached data")
			} else if detail != nil {
				updated := false
				for i, existing := range nodeDetails {
					if existing != nil && existing.Node == refreshNode {
						nodeDetails[i] = detail
						updated = true
						break
					}
				}
				if !updated {
					nodeDetails = append(nodeDetails, detail)
				}
				// Mark data source as live for this request so that future debugging
				// clearly indicates that fresh data was fetched.
				nodeCacheAgeSeconds = 0
				nodeLastUpdate[refreshNode] = time.Now().In(nodeLocation).Format(nodeTimeLayout)
			}
		}
	}

	// Build Templ data
	nodesData := components.AdminNodesData{
		Username:            getUsernameFromSession(r),
		Lang:                i18n.GetLanguage(r),
		CSRFToken:           getCSRFTokenFromContext(r),
		CurrentPath:         "/admin/nodes",
		ProxmoxConnected:    proxmoxConnected,
		Error:               errMsg,
		NodeCacheAgeSeconds: nodeCacheAgeSeconds,
		NodeLastUpdate:      nodeLastUpdate,
	}

	// Convert node details
	nodesData.NodeDetails = make([]components.NodeDetail, 0, len(nodeDetails))
	for _, nd := range nodeDetails {
		if nd == nil {
			continue
		}
		nodesData.NodeDetails = append(nodesData.NodeDetails, components.NodeDetail{
			Node:      nd.Node,
			Status:    nd.Status,
			CPU:       nd.CPU,
			MaxCPU:    nd.MaxCPU,
			Memory:    float64(nd.Memory),
			MaxMemory: float64(nd.MaxMemory),
			Disk:      float64(nd.Disk),
			MaxDisk:   float64(nd.MaxDisk),
		})
	}

	T := getTranslationFunc(r)
	if err := components.AdminNodesPage(nodesData, T).Render(r.Context(), w); err != nil {
		log.Error().Err(err).Msg("Failed to render admin nodes page")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// getNodeDetailsOptimized delegates to the shared resty helper for node collection.
func (h *AdminOptimizedHandler) getNodeDetailsOptimized(ctx context.Context, restyClient *proxmox.RestyClient) ([]*proxmox.NodeDetails, error) {
	return proxmox.FetchAllNodeDetailsResty(ctx, restyClient)
}

// AdminPageHandler redirects to the first admin page (appinfo)
func (h *AdminOptimizedHandler) AdminPageHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	http.Redirect(w, r, "/admin/appinfo", http.StatusSeeOther)
}

// AppInfoPageHandler renders the application info page
func (h *AdminOptimizedHandler) AppInfoPageHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("AppInfoPageHandler", r)

	// Collect build information
	buildInfo := map[string]interface{}{
		"version":   constants.AppVersion,
		"goVersion": runtime.Version(),
		"goOS":      runtime.GOOS,
		"goArch":    runtime.GOARCH,
	}

	// Collect environment information (safe variables only - no secrets)
	safeEnvVars := []string{
		"LOG_LEVEL",
		"PROXMOX_URL",
		"PROXMOX_VERIFY_SSL",
		"PVMSS_ENV",
		"PVMSS_OFFLINE",
		"PVMSS_SETTINGS_PATH",
	}

	envInfo := make(map[string]string)
	for _, key := range safeEnvVars {
		if val := os.Getenv(key); val != "" {
			envInfo[key] = val
		}
	}

	// Detect environment using PVMSS_ENV
	environment := "production"
	isOffline := os.Getenv("PVMSS_OFFLINE") == "true"

	if isOffline {
		environment = "offline"
	} else if !utils.IsProduction() {
		environment = "development"
	}

	buildInfo["environment"] = environment
	buildInfo["environmentDetails"] = map[string]interface{}{
		"isDevelopment": environment == "development",
		"isProduction":  environment == "production",
		"isOffline":     environment == "offline",
	}

	// Environment variables (safe only)
	buildInfo["environmentVariables"] = envInfo

	// Detect Proxmox cluster information
	clusterInfo := map[string]interface{}{
		"isCluster":   false,
		"clusterName": "",
		"nodeCount":   0,
	}

	if client := h.stateManager.GetProxmoxClient(); client != nil {
		// Try to get cluster status using the new API method
		if clusterStatus, err := proxmox.GetClusterStatus(r.Context(), client); err == nil {
			clusterInfo["isCluster"] = clusterStatus.IsCluster
			clusterInfo["clusterName"] = clusterStatus.ClusterName
			clusterInfo["nodeCount"] = clusterStatus.NodeCount
			if clusterStatus.IsCluster {
				log.Info().
					Str("cluster_name", clusterStatus.ClusterName).
					Int("nodes", clusterStatus.NodeCount).
					Msg("Proxmox cluster detected via /cluster/status")
			} else {
				log.Info().Msg("Proxmox standalone mode detected via /cluster/status")
			}
		} else {
			// Fallback to the old method using cluster name from ticket
			log.Warn().
				Err(err).
				Str("component", "admin_nodes").
				Str("operation", "get_cluster_status").
				Str("fallback", "cluster_name_detection").
				Msg("Failed to get cluster status, using fallback")
			clusterName := client.GetClusterName()
			if clusterName != "" {
				clusterInfo["isCluster"] = true
				clusterInfo["clusterName"] = clusterName
				log.Info().Str("cluster_name", clusterName).Msg("Proxmox cluster detected via fallback method")
			}
		}
	}

	buildInfo["clusterInfo"] = clusterInfo

	// Build Templ data
	appInfoData := components.AdminAppInfoData{
		Username:  getUsernameFromSession(r),
		Lang:      i18n.GetLanguage(r),
		CSRFToken: getCSRFTokenFromContext(r),
		BuildInfo: components.BuildInfo{
			Version:              constants.AppVersion,
			Environment:          environment,
			GoVersion:            runtime.Version(),
			GoOS:                 runtime.GOOS,
			GoArch:               runtime.GOARCH,
			EnvironmentVariables: envInfo,
		},
	}

	// Add cluster info if available
	if clusterInfo["isCluster"].(bool) {
		appInfoData.BuildInfo.ClusterInfo = &components.ClusterInfo{
			IsCluster:   true,
			ClusterName: clusterInfo["clusterName"].(string),
			NodeCount:   clusterInfo["nodeCount"].(int),
		}
	}

	T := getTranslationFunc(r)
	log.Info().Msg("Rendering Application Info page")
	if err := components.AdminAppInfoPage(appInfoData, T).Render(r.Context(), w); err != nil {
		log.Error().Err(err).Msg("Failed to render admin appinfo page")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// ProxmoxTicketTestPageHandler renders the Proxmox ticket test page
func (h *AdminOptimizedHandler) ProxmoxTicketTestPageHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("ProxmoxTicketTestPageHandler", r)
	// Get Proxmox host URL from client
	var proxmoxHost string
	var authMethod string
	client := h.stateManager.GetProxmoxClient()
	if client != nil {
		proxmoxHost = client.GetApiUrl()
		// Remove protocol and port to get just the hostname
		if strings.HasPrefix(proxmoxHost, "https://") {
			proxmoxHost = strings.TrimPrefix(proxmoxHost, "https://")
		} else if strings.HasPrefix(proxmoxHost, "http://") {
			proxmoxHost = strings.TrimPrefix(proxmoxHost, "http://")
		}
		// Remove port if present
		if host, _, err := net.SplitHostPort(proxmoxHost); err == nil {
			proxmoxHost = host
		}

		// Check authentication method
		if os.Getenv("PROXMOX_API_TOKEN_NAME") != "" && os.Getenv("PROXMOX_API_TOKEN_VALUE") != "" {
			authMethod = "API Token"
		} else if os.Getenv("PROXMOX_USER") != "" && os.Getenv("PROXMOX_PASSWORD") != "" {
			authMethod = "Username/Password"
		} else {
			authMethod = "Unknown"
		}
	}

	// Build Templ data
	ticketTestData := components.AdminTicketTestData{
		Username:    getUsernameFromSession(r),
		Lang:        i18n.GetLanguage(r),
		CSRFToken:   getCSRFTokenFromContext(r),
		ProxmoxHost: proxmoxHost,
		AuthMethod:  authMethod,
	}

	T := getTranslationFunc(r)
	if err := components.AdminTicketTestPage(ticketTestData, T).Render(r.Context(), w); err != nil {
		log.Error().Err(err).Msg("Failed to render admin ticket test page")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// ProxmoxTicketTestFormHandler handles POST from admin_ticket_test.html to test Proxmox authentication
func (h *AdminOptimizedHandler) ProxmoxTicketTestFormHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("ProxmoxTicketTestFormHandler", r)
	if !ValidateMethodAndParseForm(w, r, http.MethodPost) {
		return
	}

	// For now, just redirect with a success message
	localizer := i18n.GetLocalizerFromRequest(r)

	// Build Templ data
	ticketTestData := components.AdminTicketTestData{
		Username:   getUsernameFromSession(r),
		Lang:       i18n.GetLanguage(r),
		CSRFToken:  getCSRFTokenFromContext(r),
		Success:    true,
		SuccessMsg: i18n.Localize(localizer, "Admin.TicketTest.Success"),
	}

	T := getTranslationFunc(r)
	if err := components.AdminTicketTestPage(ticketTestData, T).Render(r.Context(), w); err != nil {
		log.Error().Err(err).Msg("Failed to render admin ticket test page")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
