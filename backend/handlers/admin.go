package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/julienschmidt/httprouter"

	"pvmss/constants"
	"pvmss/proxmox"
	"pvmss/state"
)

// AdminHandler handles administration routes
type AdminHandler struct {
	stateManager state.StateManager
}

// NodesPageHandler renders the Nodes admin page
func (h *AdminHandler) NodesPageHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("NodesPageHandler", r)

	// Proxmox connection status from background monitor
	proxmoxConnected, _ := h.stateManager.GetProxmoxStatus()
	client := h.stateManager.GetProxmoxClient()
	var nodeDetails []*proxmox.NodeDetails
	var errMsg string

	if proxmoxConnected && client != nil {
		// Check if client is our custom Client type
		if pc, ok := client.(*proxmox.Client); ok {
			// Use a shorter timeout to avoid long blocking even if status recently changed
			ctx, cancel := context.WithTimeout(r.Context(), constants.ShortContextTimeout)
			defer cancel()
			nodes, err := proxmox.GetNodeNamesWithContext(ctx, pc)
			if err != nil {
				log.Warn().Err(err).Msg("Unable to retrieve Proxmox nodes")
				errMsg = "Failed to retrieve nodes"
			} else {
				var wg sync.WaitGroup
				detailsChan := make(chan *proxmox.NodeDetails, len(nodes))

				for _, nodeName := range nodes {
					wg.Add(1)
					go func(name string) {
						defer wg.Done()
						nd, nErr := proxmox.GetNodeDetails(pc, name)
						if nErr != nil {
							log.Warn().Err(nErr).Str("node", name).Msg("Failed to retrieve node details; skipping node")
							return
						}
						detailsChan <- nd
					}(nodeName)
				}

				wg.Wait()
				close(detailsChan)

				for detail := range detailsChan {
					nodeDetails = append(nodeDetails, detail)
				}
			}
		} else {
			log.Warn().Msg("Proxmox client is not the expected type")
			errMsg = "Invalid client type"
		}
	} else {
		log.Warn().Msg("Proxmox client is not initialized; rendering page without live node data")
	}

	data := AdminPageDataWithMessage("Node Management", "nodes", "", errMsg)
	data["ProxmoxConnected"] = proxmoxConnected
	data["NodeDetails"] = nodeDetails
	renderTemplateInternal(w, r, "admin_nodes", data)
}

// NewAdminHandler creates a new instance of AdminHandler
func NewAdminHandler(sm state.StateManager) *AdminHandler {
	return &AdminHandler{stateManager: sm}
}

// AdminPageHandler handles the administration page
func (h *AdminHandler) AdminPageHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("AdminPageHandler", r)
	log.Debug().Msg("Rendering admin dashboard")

	// Proxmox connection status from background monitor
	proxmoxConnected, _ := h.stateManager.GetProxmoxStatus()

	data := AdminPageDataWithMessage("Admin Dashboard", "dashboard", "", "")
	data["ProxmoxConnected"] = proxmoxConnected
	renderTemplateInternal(w, r, "admin_base", data)
}

// ProxmoxTicketTestPageHandler renders the Proxmox ticket test page
func (h *AdminHandler) ProxmoxTicketTestPageHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
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

	data := AdminPageDataWithMessage("Proxmox Ticket Test", "ticket-test", "", "")
	data["ProxmoxHost"] = proxmoxHost
	data["AuthMethod"] = authMethod
	renderTemplateInternal(w, r, "admin_ticket_test", data)
}

// ProxmoxTicketTestFormHandler handles POST from admin_ticket_test.html to test Proxmox authentication
func (h *AdminHandler) ProxmoxTicketTestFormHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	if !ValidateMethodAndParseForm(w, r, http.MethodPost) {
		return
	}

	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")

	if username == "" {
		data := AdminPageDataWithMessage("Proxmox Ticket Test", "ticket-test", "", "Username is required")
		data["ProxmoxHost"] = r.FormValue("proxmox_host")
		renderTemplateInternal(w, r, "admin_ticket_test", data)
		return
	}

	// Check if the main client is using API token authentication
	mainClient := h.stateManager.GetProxmoxClient()
	if mainClient != nil {
		// If main client exists and is working with API tokens, we can't test username/password
		// because API token auth doesn't support the login endpoint
		data := AdminPageDataWithMessage("Proxmox Ticket Test", "ticket-test", "", "Your Proxmox configuration uses API token authentication. Username/password testing is not available with API tokens.")
		data["ProxmoxHost"] = r.FormValue("proxmox_host")
		data["AuthMethod"] = "API Token"
		data["Username"] = username
		renderTemplateInternal(w, r, "admin_ticket_test", data)
		return
	}

	// Test the authentication
	// Get the SSL verification setting from environment
	insecureSkipVerify := os.Getenv("PROXMOX_VERIFY_SSL") == "false"

	testClient, err := proxmox.NewClientCookieAuth("https://"+r.FormValue("proxmox_host")+":8006/api2/json", insecureSkipVerify)
	if err != nil {
		data := AdminPageDataWithMessage("Proxmox Ticket Test", "ticket-test", "", "Failed to create test client: "+err.Error())
		data["ProxmoxHost"] = r.FormValue("proxmox_host")
		renderTemplateInternal(w, r, "admin_ticket_test", data)
		return
	}

	testClient.SetTimeout(30 * time.Second)

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Try to login with username and password
	err = testClient.Login(ctx, username, password, "")
	if err != nil {
		data := AdminPageDataWithMessage("Proxmox Ticket Test", "ticket-test", "", "Authentication failed: "+err.Error())
		data["ProxmoxHost"] = r.FormValue("proxmox_host")
		renderTemplateInternal(w, r, "admin_ticket_test", data)
		return
	}

	// Test the authentication by making a simple API call
	var result map[string]interface{}
	err = testClient.GetJSON(ctx, "/nodes", &result)
	if err != nil {
		data := AdminPageDataWithMessage("Proxmox Ticket Test", "ticket-test", "", "Ticket validation failed: "+err.Error())
		data["ProxmoxHost"] = r.FormValue("proxmox_host")
		renderTemplateInternal(w, r, "admin_ticket_test", data)
		return
	}

	// Success - show the results
	data := AdminPageDataWithMessage("Proxmox Ticket Test", "ticket-test", "Authentication successful! Ticket obtained and validated.", "")
	data["ProxmoxHost"] = r.FormValue("proxmox_host")
	data["Username"] = username

	// Extract ticket information
	pveAuthCookie := testClient.GetPVEAuthCookie()
	csrfToken := testClient.GetCSRFPreventionToken()

	// Parse ticket details (PVE tickets are JWT-like tokens)
	ticketDetails := map[string]interface{}{
		"Length":       len(pveAuthCookie),
		"Format":       "JWT-like token",
		"ContainsDots": strings.Count(pveAuthCookie, "."),
		"Timestamp":    time.Now().Format("2006-01-02 15:04:05 MST"),
		"ValidFor":     "2 hours",
		"Host":         r.FormValue("proxmox_host"),
	}

	// Try to decode ticket payload if it's a JWT-like token
	if parts := strings.Split(pveAuthCookie, "."); len(parts) >= 2 {
		if payload, err := base64.RawURLEncoding.DecodeString(parts[1]); err == nil {
			var ticketPayload map[string]interface{}
			if err := json.Unmarshal(payload, &ticketPayload); err == nil {
				ticketDetails["DecodedPayload"] = ticketPayload
			}
		}
	}

	data["TicketInfo"] = map[string]interface{}{
		"PVEAuthCookie":       pveAuthCookie,
		"CSRFPreventionToken": csrfToken,
		"TicketDetails":       ticketDetails,
		"ApiResponse":         result,
	}
	renderTemplateInternal(w, r, "admin_ticket_test", data)
}

// RegisterRoutes registers administration routes
func (h *AdminHandler) RegisterRoutes(router *httprouter.Router) {
	// Register main admin dashboard (protected with admin privileges)
	router.GET("/admin", HandlerFuncToHTTPrHandle(RequireAdminAuth(func(w http.ResponseWriter, r *http.Request) {
		h.AdminPageHandler(w, r, httprouter.ParamsFromContext(r.Context()))
	})))

	// Additional admin subpages (protected with admin privileges)
	router.GET("/admin/nodes", HandlerFuncToHTTPrHandle(RequireAdminAuth(func(w http.ResponseWriter, r *http.Request) {
		h.NodesPageHandler(w, r, httprouter.ParamsFromContext(r.Context()))
	})))

	// Proxmox ticket test routes
	router.GET("/admin/ticket-test", HandlerFuncToHTTPrHandle(RequireAdminAuth(func(w http.ResponseWriter, r *http.Request) {
		h.ProxmoxTicketTestPageHandler(w, r, httprouter.ParamsFromContext(r.Context()))
	})))

	// Admin ticket test form with CSRF protection
	router.POST("/admin/ticket-test", SecureFormHandler("ProxmoxTicketTest",
		HandlerFuncToHTTPrHandle(RequireAdminAuth(func(w http.ResponseWriter, r *http.Request) {
			h.ProxmoxTicketTestFormHandler(w, r, httprouter.ParamsFromContext(r.Context()))
		})),
	))
}
