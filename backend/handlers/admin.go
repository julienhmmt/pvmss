package handlers

import (
	"context"
	"net/http"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/julienschmidt/httprouter"

	"pvmss/constants"
	"pvmss/proxmox"
	"pvmss/state"
)

// NodeError captures an error along with the node name
type NodeError struct {
	Node  string
	Error error
}

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

	// Admin nodes page (optimized)
	router.GET("/admin/nodes", RequireAuthHandle(h.NodesPageHandlerOptimized))

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
				}
			}
		} else {
			log.Warn().Msg("Proxmox credentials not configured")
			errMsg = "Proxmox credentials missing"
		}
	} else {
		log.Warn().Msg("Proxmox client is not initialized; rendering page without live node data")
		errMsg = "Proxmox connection unavailable"
	}

	// Build template data with optimized builder pattern
	data := NewTemplateDataWithOptions("",
		WithAdminActive("nodes"),
		WithAuth(r),
		WithProxmoxStatus(h.stateManager),
		WithMessages(r),
		WithData("NodeDetails", nodeDetails),
		WithData("Error", errMsg),
	).ToMap()

	renderTemplateInternal(w, r, "admin_nodes", data)
}

// getNodeDetailsOptimized retrieves node details with batch processing and caching optimizations
// Returns ALL nodes including offline ones in cluster mode
func (h *AdminOptimizedHandler) getNodeDetailsOptimized(ctx context.Context, restyClient *proxmox.RestyClient) ([]*proxmox.NodeDetails, error) {
	log := CreateHandlerLogger("getNodeDetailsOptimized", nil)

	// First, get node names (fast operation)
	nodes, err := proxmox.GetNodeNamesResty(ctx, restyClient)
	if err != nil {
		return nil, err
	}

	log.Info().Int("node_count", len(nodes)).Msg("Retrieved node names")

	if len(nodes) == 0 {
		return []*proxmox.NodeDetails{}, nil
	}

	// Check if we're in cluster mode by trying to get cluster status
	clusterInfo, clusterErr := proxmox.GetClusterStatusResty(ctx, restyClient)
	isClusterMode := clusterErr == nil && clusterInfo != nil && clusterInfo.IsCluster

	// Use optimized concurrent processing with semaphore
	const maxConcurrent = 8 // Increased from original for better performance
	semaphore := make(chan struct{}, maxConcurrent)

	var wg sync.WaitGroup
	detailsChan := make(chan *proxmox.NodeDetails, len(nodes))
	errorChan := make(chan NodeError, len(nodes))

	// Process nodes concurrently with controlled concurrency
	for _, nodeName := range nodes {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Create individual context with shorter timeout for each node
			nodeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()

			nd, nErr := proxmox.GetNodeDetailsResty(nodeCtx, restyClient, name)
			if nErr != nil {
				log.Warn().Err(nErr).Str("node", name).Msg("Failed to retrieve node details (optimized)")
				errorChan <- NodeError{Node: name, Error: nErr}
				
				// In cluster mode, create fallback NodeDetails for offline nodes
				if isClusterMode {
					fallbackDetails := &proxmox.NodeDetails{
						Node:   name,
						Status: "offline", // Mark as offline
						CPU:    0,
						MaxCPU: 0,
						Sockets: 0,
						Memory: 0,
						MaxMemory: 0,
						Disk: 0,
						MaxDisk: 0,
						Uptime: 0,
					}
					detailsChan <- fallbackDetails
					log.Info().Str("node", name).Msg("Created fallback details for offline node in cluster mode")
				}
				return
			}

			detailsChan <- nd
		}(nodeName)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(detailsChan)
	close(errorChan)

	// Collect results
	var nodeDetails []*proxmox.NodeDetails
	for detail := range detailsChan {
		nodeDetails = append(nodeDetails, detail)
	}

	// Log errors (but don't fail the entire operation)
	errorCount := 0
	fallbackCount := 0
	for range errorChan {
		errorCount++
		if isClusterMode {
			fallbackCount++ // Each error in cluster mode creates a fallback
		}
	}
	
	if isClusterMode && fallbackCount > 0 {
		log.Info().Int("fallback_count", fallbackCount).Msg("Created fallback entries for offline nodes in cluster mode")
	} else if errorCount > 0 {
		log.Warn().Int("error_count", errorCount).Int("success_count", len(nodeDetails)).Msg("Some node details failed to load")
	}

	// Sort nodes alphabetically by name
	sort.Slice(nodeDetails, func(i, j int) bool {
		return nodeDetails[i].Node < nodeDetails[j].Node
	})

	log.Info().
		Int("node_details_count", len(nodeDetails)).
		Int("total_nodes", len(nodes)).
		Int("error_count", errorCount).
		Bool("cluster_mode", isClusterMode).
		Msg("Successfully fetched node details with optimization")

	return nodeDetails, nil
}
