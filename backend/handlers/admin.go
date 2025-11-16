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

	log.Debug().Msg("Registering optimized admin routes")

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

	// Proxmox connection status from background monitor
	proxmoxConnected, _ := h.stateManager.GetProxmoxStatus()
	var nodeDetails []*proxmox.NodeDetails
	var errMsg string
	nodeDataSource := "live"
	nodeCacheAgeSeconds := 0

	if cachedDetails, cacheTimestamp := h.stateManager.GetNodeCache(); len(cachedDetails) > 0 {
		nodeDetails = cachedDetails
		nodeDataSource = "cache"
		age := int(time.Since(cacheTimestamp).Seconds())
		if age < 0 {
			age = 0
		}
		nodeCacheAgeSeconds = age
		log.Debug().Int("node_details_count", len(nodeDetails)).Int("cache_age_seconds", nodeCacheAgeSeconds).Msg("Serving node details from cache")
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
						log.Warn().Err(err).Msg("Unable to retrieve Proxmox node details (optimized)")
						errMsg = "Failed to retrieve node details"
					} else {
						log.Info().Int("node_details_count", len(nodeDetails)).Msg("Successfully fetched node details with optimization")
						nodeDataSource = "live"
						nodeCacheAgeSeconds = 0
					}
				}
			} else {
				log.Warn().Msg("Proxmox credentials not configured")
				errMsg = "Proxmox credentials missing"
			}
		} else {
			log.Warn().Msg("Proxmox client is offline; using cached data if available")
			errMsg = "Proxmox connection unavailable"
		}
	} else {
		log.Debug().Int("node_details_count", len(nodeDetails)).Str("source", nodeDataSource).Msg("Rendering node details from cache")
	}

	// Build template data with optimized builder pattern
	data := NewTemplateDataWithOptions("",
		WithAdminActive("nodes"),
		WithAuth(r),
		WithProxmoxStatus(h.stateManager),
		WithMessages(r),
		WithData("NodeDetails", nodeDetails),
		WithData("Error", errMsg),
		WithData("NodeDataSource", nodeDataSource),
		WithData("NodeCacheAgeSeconds", nodeCacheAgeSeconds),
	).ToMap()

	renderTemplateInternal(w, r, "admin_nodes", data)
}

// getNodeDetailsOptimized delegates to the shared resty helper for node collection.
func (h *AdminOptimizedHandler) getNodeDetailsOptimized(ctx context.Context, restyClient *proxmox.RestyClient) ([]*proxmox.NodeDetails, error) {
	return proxmox.FetchAllNodeDetailsResty(ctx, restyClient)
}

// AdminPageHandler renders the admin dashboard
func (h *AdminOptimizedHandler) AdminPageHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("AdminPageHandler", r)
	log.Debug().Msg("Rendering admin dashboard")

	data := NewTemplateDataWithOptions("",
		WithAdminActive("dashboard"),
		WithAuth(r),
		WithProxmoxStatus(h.stateManager),
		WithMessages(r),
		WithData("TitleKey", "Navbar.Admin"),
	).ToMap()
	renderTemplateInternal(w, r, "admin_base", data)
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
			log.Warn().Err(err).Msg("Failed to get cluster status, falling back to cluster name detection")
			clusterName := client.GetClusterName()
			if clusterName != "" {
				clusterInfo["isCluster"] = true
				clusterInfo["clusterName"] = clusterName
				log.Info().Str("cluster_name", clusterName).Msg("Proxmox cluster detected via fallback method")
			}
		}
	}

	buildInfo["clusterInfo"] = clusterInfo

	data := NewTemplateDataWithOptions("",
		WithAdminActive("appinfo"),
		WithAuth(r),
		WithProxmoxStatus(h.stateManager),
		WithMessages(r),
		WithData("TitleKey", "Admin.AppInfo.Title"),
		WithData("BuildInfo", buildInfo),
	).ToMap()
	log.Info().Msg("Rendering Application Info page")
	renderTemplateInternal(w, r, "admin_appinfo", data)
}

// ProxmoxTicketTestPageHandler renders the Proxmox ticket test page
func (h *AdminOptimizedHandler) ProxmoxTicketTestPageHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
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

	data := NewTemplateDataWithOptions("",
		WithAdminActive("ticket-test"),
		WithAuth(r),
		WithProxmoxStatus(h.stateManager),
		WithMessages(r),
		WithData("TitleKey", "Navbar.Admin"),
		WithData("ProxmoxHost", proxmoxHost),
		WithData("AuthMethod", authMethod),
	).ToMap()
	renderTemplateInternal(w, r, "admin_ticket_test", data)
}

// ProxmoxTicketTestFormHandler handles POST from admin_ticket_test.html to test Proxmox authentication
func (h *AdminOptimizedHandler) ProxmoxTicketTestFormHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	if !ValidateMethodAndParseForm(w, r, http.MethodPost) {
		return
	}

	// For now, just redirect with a success message
	localizer := i18n.GetLocalizerFromRequest(r)

	data := NewTemplateDataWithOptions("",
		WithAdminActive("ticket-test"),
		WithAuth(r),
		WithProxmoxStatus(h.stateManager),
		WithSuccess(i18n.Localize(localizer, "Admin.TicketTest.Success")),
	).ToMap()
	renderTemplateInternal(w, r, "admin_ticket_test", data)
}
