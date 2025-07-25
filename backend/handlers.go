package main

import (
	"net/http"
	"strings"

	"golang.org/x/crypto/bcrypt"
	"pvmss/logger"
	"pvmss/proxmox"
	"pvmss/state"
)

// indexHandler handles requests to the root ("/") path.
// It serves the main landing page of the application.
func indexHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// Check if user is authenticated
	if isAuthenticated(r) {
		// If authenticated, redirect to admin page
		http.Redirect(w, r, "/admin", http.StatusFound)
		return
	}

	// If not authenticated, show the index page
	renderTemplate(w, r, "index.html", nil)
}

// searchHandler handles VM search requests.
// It processes POST requests containing search criteria (VMID or name),
// fetches results from the Proxmox API, and renders the search results page.
func searchHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse form data
	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	// Get search parameters
	vmid := strings.TrimSpace(r.FormValue("vmid"))
	name := strings.TrimSpace(r.FormValue("name"))

	// Validate input
	if vmid == "" && name == "" {
		http.Error(w, "VMID or name is required", http.StatusBadRequest)
		return
	}

	// Get VMs from Proxmox
	proxmoxClient := state.GetProxmoxClient()
	if proxmoxClient == nil {
		http.Error(w, "Failed to connect to Proxmox API", http.StatusInternalServerError)
		return
	}

	// Get all VMs across all nodes
	vms, err := proxmox.GetVMsWithContext(r.Context(), proxmoxClient)
	if err != nil {
		http.Error(w, "Failed to fetch VMs: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var results []map[string]interface{}

	// Filter VMs by search criteria if provided
	for _, vm := range vms {
		// Check if VM matches search criteria
		vmID, _ := vm["vmid"].(string)
		vmName, _ := vm["name"].(string)

		// If search criteria is provided, filter VMs
		if vmid != "" && vmID != vmid {
			continue
		}
		if name != "" && !strings.Contains(strings.ToLower(vmName), strings.ToLower(name)) {
			continue
		}

		results = append(results, vm)
	}

	// Prepare template data
	data := map[string]interface{}{
		"Results": results,
	}

	renderTemplate(w, r, "search.html", data)
}

// createVmHandler handles the display of the VM creation page.
// It loads application settings to provide the template with necessary data like available ISOs, networks, and tags.
func createVmHandler(w http.ResponseWriter, r *http.Request) {
	proxmoxClient := state.GetProxmoxClient()
	if proxmoxClient == nil {
		http.Error(w, "Failed to connect to Proxmox API", http.StatusInternalServerError)
		return
	}

	// Get available nodes
	nodes, err := proxmox.GetNodeNames(proxmoxClient)
	if err != nil {
		http.Error(w, "Failed to fetch nodes: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Get tags from state
	tags := state.GetTags()

	// Prepare template data
	data := map[string]interface{}{
		"Nodes": nodes,
		"Tags":  tags,
	}

	renderTemplate(w, r, "create_vm.html", data)
}

// adminHandler renders the main administration page.
// It fetches comprehensive data from both the Proxmox API (nodes, storage, ISOs, VMBRs)
// and the local settings.json file to populate the template with all necessary configuration options.
func adminHandler(w http.ResponseWriter, r *http.Request) {
	// Get Proxmox client from state
	proxmoxClient := state.GetProxmoxClient()
	if proxmoxClient == nil {
		http.Error(w, "Failed to connect to Proxmox API", http.StatusInternalServerError)
		return
	}

	// Get all nodes
	nodes, err := proxmox.GetNodeNames(proxmoxClient)
	if err != nil {
		http.Error(w, "Failed to fetch nodes: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Get storage information
	var storageList []Storage
	storages, err := getStorages()
	if err != nil {
		logger.Get().Error().Err(err).Msg("Failed to fetch storage information")
	} else {
		storageList = storages // Use the returned storages
	}

	// Get ISO files
	var isoFiles []string
	for _, storage := range storageList {
		if storage.Content == "iso" {
			node := storage.Nodes
			storageName := storage.Name
			if node != "" && storageName != "" {
				// Get ISO list for this storage
				isoList, err := proxmox.GetISOList(proxmoxClient, node, storageName)
				if err != nil {
					logger.Get().Error().Err(err).
						Str("node", node).
						Str("storage", storageName).
						Msg("Failed to fetch ISO files")
					continue
				}

				// Extract ISO file names from the response
				if data, ok := isoList["data"].([]interface{}); ok {
					for _, item := range data {
						if iso, ok := item.(map[string]interface{}); ok {
							if volname, ok := iso["volid"].(string); ok && strings.HasSuffix(volname, ".iso") {
								isoFiles = append(isoFiles, volname)
							}
						}
					}
				}
			}
		}
	}

	// Get network interfaces (VMBRs)
	var vmbrs []string
	if len(nodes) > 0 {
		vmbrList, err := proxmox.GetVMBRs(proxmoxClient, nodes[0]) // Get from first node
		if err != nil {
			logger.Get().Error().Err(err).Msg("Failed to fetch network interfaces")
		} else {
			for _, vmbr := range vmbrList {
				if vmbrMap, ok := vmbr.(map[string]interface{}); ok {
					if name, ok := vmbrMap["iface"].(string); ok && strings.HasPrefix(name, "vmbr") {
						vmbrs = append(vmbrs, name)
					}
				}
			}
		}
	}

	// Prepare template data
	data := map[string]interface{}{
		"Nodes":     nodes,
		"Storages":  storageList,
		"ISOs":      isoFiles,
		"VMBRs":     vmbrs,
		"PageTitle": "Admin",
	}

	renderTemplate(w, r, "admin.html", data)
}

// storagePageHandler handles requests to the storage management page
func storagePageHandler(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, r, "storage.html", nil)
}

// isoPageHandler handles requests to the ISO management page
func isoPageHandler(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, r, "iso.html", nil)
}

// vmbrPageHandler handles requests to the network management page
func vmbrPageHandler(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, r, "vmbr.html", nil)
}

// healthHandler handles health check requests
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// loginHandler handles user authentication.
// For GET requests, it displays the login page.
// For POST requests, it validates the password against the stored bcrypt hash,
// authenticates the session, and redirects to the admin page on success.
func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		renderTemplate(w, r, "login.html", nil)
		return
	}

	// Handle POST request
	password := r.FormValue("password")
	if password == "" {
		http.Error(w, "Password is required", http.StatusBadRequest)
		return
	}

	// Get admin password hash from state
	adminPasswordHash := state.GetAdminPassword()
	if adminPasswordHash == "" {
		http.Error(w, "Admin password not configured", http.StatusInternalServerError)
		return
	}

	// Compare the provided password with the stored hash
	err := bcrypt.CompareHashAndPassword([]byte(adminPasswordHash), []byte(password))
	if err != nil {
		http.Error(w, "Invalid password", http.StatusUnauthorized)
		return
	}

	// Authentication successful, create session
	sm := state.GetSessionManager()
	if sm != nil {
		sm.Put(r.Context(), "authenticated", true)
		http.Redirect(w, r, "/admin", http.StatusFound)
	} else {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// logoutHandler handles user logout.
// It destroys the current session and redirects the user to the homepage.
func logoutHandler(w http.ResponseWriter, r *http.Request) {
	sm := state.GetSessionManager()
	if sm != nil {
		sm.Destroy(r.Context())
		http.Redirect(w, r, "/", http.StatusFound)
	} else {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// authMiddleware is a middleware that checks if the user is authenticated.
// If not, it redirects to the login page.
func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth for public paths
		publicPaths := []string{"/", "/login", "/health", "/static"}
		for _, path := range publicPaths {
			if strings.HasPrefix(r.URL.Path, path) {
				next.ServeHTTP(w, r)
				return
			}
		}

		// Check if user is authenticated
		if !isAuthenticated(r) {
			http.Redirect(w, r, "/login?redirect="+r.URL.Path, http.StatusFound)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Helper function to check if a request is authenticated
func isAuthenticated(r *http.Request) bool {
	sm := state.GetSessionManager()
	if sm == nil {
		return false
	}
	return sm.GetBool(r.Context(), "authenticated")
}
