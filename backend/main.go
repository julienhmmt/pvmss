package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"

	"pvmss/backend/logger"
	"pvmss/backend/proxmox"
)

// Application globals
var (
	templates      *template.Template
	sessionManager *scs.SessionManager
	proxmoxClient  *proxmox.Client
)

// initLogger initializes the logger with the configured log level
func initLogger() {
	level := os.Getenv("LOG_LEVEL")
	if level == "" {
		level = "info" // default to info level if not set
	}
	logger.Init(level)
}

// validateEnvironment checks for required environment variables
func validateEnvironment() error {
	requiredVars := []string{
		"PROXMOX_URL",
		"PROXMOX_API_TOKEN_NAME",
		"PROXMOX_API_TOKEN_VALUE",
	}

	for _, v := range requiredVars {
		if os.Getenv(v) == "" {
			return fmt.Errorf("required environment variable %s is not set", v)
		}
	}

	proxmoxURL := os.Getenv("PROXMOX_URL")
	if proxmoxURL == "" {
		return fmt.Errorf("PROXMOX_URL environment variable is required")
	}

	logger.Get().Info().Str("PROXMOX_URL", proxmoxURL).Msg("Proxmox URL loaded")

	return nil
}

// initProxmoxClient creates and configures the Proxmox API client
func initProxmoxClient() (*proxmox.Client, error) {
	proxmoxURL := os.Getenv("PROXMOX_URL")
	proxmoxAPITokenName := os.Getenv("PROXMOX_API_TOKEN_NAME")
	proxmoxAPITokenValue := os.Getenv("PROXMOX_API_TOKEN_VALUE")
	insecureSkipVerify := os.Getenv("PROXMOX_VERIFY_SSL") == "false"

	logger.Get().Info().
		Str("url", proxmoxURL).
		Str("tokenName", proxmoxAPITokenName).
		Bool("insecureSkipVerify", insecureSkipVerify).
		Msg("Initializing Proxmox API client")

	client, err := proxmox.NewClientWithOptions(
		proxmoxURL,
		proxmoxAPITokenName,
		proxmoxAPITokenValue,
		insecureSkipVerify,
	)

	if err != nil {
		logger.Get().Error().Err(err).Msg("Failed to initialize Proxmox client")
		return nil, err
	}

	logger.Get().Info().Msg("Proxmox client initialized successfully")
	return client, nil
}

// Settings holds the application settings
type Settings struct {
	Tags  []string `json:"tags"`
	RAM   struct {
		Min int `json:"min"`
		Max int `json:"max"`
	} `json:"ram"`
	CPU struct {
		Min int `json:"min"`
		Max int `json:"max"`
	} `json:"cpu"`
	Sockets struct {
		Min int `json:"min"`
		Max int `json:"max"`
	} `json:"sockets"`
	ISOs  []string `json:"isos"`
	VMBRs []string `json:"vmbrs"`
}

var appSettings Settings

// getTranslations returns a map of translations for the given language
func getTranslations(lang string) map[string]string {
	translations := map[string]map[string]string{
		"en": {
			"CreateVMTitle":    "Create New Virtual Machine",
			"NoISOsMessage":    "No ISO images available. Please contact an administrator.",
			"NoBridgesMessage": "No network bridges available. Please contact an administrator.",
		},
		"fr": {
			"CreateVMTitle":    "Créer une nouvelle machine virtuelle",
			"NoISOsMessage":    "Aucune image ISO disponible. Veuillez contacter un administrateur.",
			"NoBridgesMessage": "Aucun pont réseau disponible. Veuillez contacter un administrateur.",
		},
		// Add more languages as needed
	}

	// Default to English if the requested language is not available
	if _, exists := translations[lang]; !exists {
		lang = "en"
	}

	return translations[lang]
}

// loadSettings loads the application settings from settings.json
func loadSettings() error {
	file, err := os.Open("settings.json")
	if err != nil {
		return fmt.Errorf("error opening settings file: %w", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	err = decoder.Decode(&appSettings)
	if err != nil {
		return fmt.Errorf("error decoding settings: %w", err)
	}

	return nil
}

// initTemplates initializes the HTML templates with custom functions
func initTemplates() (*template.Template, error) {
	// Define template functions
	funcMap := template.FuncMap{
		"div": func(a, b float64) float64 {
			if b == 0 {
				return 0
			}
			return a / b
		},
		"mul": func(a, b float64) float64 {
			return a * b
		},
		"until": func(n int) []int {
			var arr []int
			for i := 0; i < n; i++ {
				arr = append(arr, i)
			}
			return arr
		},
		"contains":  strings.Contains,
		"splitList": strings.Split,
		"sub":      func(a, b int) int { return a - b },
		"add":      func(a, b int) int { return a + b },
		"sort":     func(s []string) []string {
			sorted := make([]string, len(s))
			copy(sorted, s)
			sort.Strings(sorted)
			return sorted
		},
		"humanBytes": func(b interface{}) string {
			var bytes uint64
			switch v := b.(type) {
			case float64:
				bytes = uint64(v)
			case uint64:
				bytes = v
			case int:
				bytes = uint64(v)
			case int64:
				bytes = uint64(v)
			default:
				return "N/A"
			}

			const unit = 1024
			if bytes < unit {
				return fmt.Sprintf("%d B", bytes)
			}
			div, exp := uint64(unit), 0
			for n := bytes / unit; n >= unit; n /= unit {
				div *= unit
				exp++
			}
			return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
		},
		"formatMemory": formatMemory,
	}

	// Create a new template with the layout file first
	tmpl := template.New("layout.html").Funcs(funcMap)
	
	// Parse all HTML templates in the frontend directory
	tmpl, err := tmpl.ParseGlob("../frontend/*.html")
	if err != nil {
		return nil, fmt.Errorf("error parsing templates: %w", err)
	}
	
	// Store the parsed templates in the global variable
	templates = tmpl
	
	return tmpl, nil
}

// setupServer configures the HTTP server with routes and middleware
func setupServer(ctx context.Context) *http.Server {
	// Get port from environment
	port := os.Getenv("PORT")
	if port == "" {
		port = "50000"
		logger.Get().Info().Str("port", port).Msg("Using default port")
	}

	// Create router
	r := http.NewServeMux()

	// Serve static files
	fs := http.FileServer(http.Dir("../frontend/css"))
	r.Handle("/css/", http.StripPrefix("/css/", fs))

	// Handlers
	r.HandleFunc("/", indexHandler)
	r.HandleFunc("/search", searchHandler)
	r.HandleFunc("/login", loginHandler)
	r.HandleFunc("/logout", logoutHandler)
	r.HandleFunc("/vm/details", vmDetailsHandler)
	r.HandleFunc("/vm/action", vmActionHandler)
	r.HandleFunc("/create-vm", createVmHandler)
	r.HandleFunc("/api/vm/status", apiVmStatusHandler)

	// Documentation routes
	r.HandleFunc("/docs/admin", serveDocHandler("admin"))
	r.HandleFunc("/docs/user", serveDocHandler("user"))

	authedRoutes := http.NewServeMux()
	authedRoutes.HandleFunc("/admin", adminHandler)

	r.Handle("/admin", authMiddleware(authedRoutes))
	r.HandleFunc("/storage", storagePageHandler)
	r.HandleFunc("/iso", isoPageHandler)
	r.HandleFunc("/vmbr", vmbrPageHandler)
	r.HandleFunc("/health", healthHandler)

	// API handlers
	r.HandleFunc("/api/tags", tagsHandler)
	r.HandleFunc("/api/tags/", tagsHandler)
	r.HandleFunc("/api/storage", storageHandler)
	r.HandleFunc("/api/iso/all", allIsosHandler)
	r.HandleFunc("/api/vmbr/all", allVmbrsHandler)
	r.HandleFunc("/api/settings", settingsHandler)
	r.HandleFunc("/api/iso/settings", updateIsoSettingsHandler)
	r.HandleFunc("/api/vmbr/settings", updateVmbrSettingsHandler)
	r.HandleFunc("/api/limits", limitsHandler)

	// Configure server with timeouts
	return &http.Server{
		Addr:         ":" + port,
		Handler:      sessionManager.LoadAndSave(r),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}

// startServer starts the HTTP server
func startServer(srv *http.Server) {
	logger.Get().Info().Str("addr", srv.Addr).Msg("Starting server...")
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Get().Fatal().Err(err).Msg("Server failed to start")
	}
}

func main() {
	// Setup context with cancellation for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize logger first for proper error reporting
	initLogger()

	// Load environment variables
	if err := godotenv.Load("../.env"); err != nil {
		logger.Get().Warn().Msg("Error loading .env file, relying on environment variables")
	}

	// Validate required environment variables
	if err := validateEnvironment(); err != nil {
		logger.Get().Fatal().Err(err).Msg("Environment validation failed")
	}

	// Initialize Proxmox client
	var err error
	proxmoxClient, err = initProxmoxClient()
	if err != nil {
		logger.Get().Fatal().Err(err).Msg("Failed to initialize Proxmox client")
	}

	// Initialize i18n
	initI18n()

	// Initialize templates
	tmpl, err := initTemplates()
	if err != nil {
		logger.Get().Fatal().Err(err).Msg("Failed to initialize templates")
	}
	templates = tmpl

	// Initialize session manager
	sessionManager = scs.New()
	sessionManager.Lifetime = 24 * time.Hour

	// Setup HTTP server with proper timeouts and handlers
	server := setupServer(ctx)

	// Start server in a goroutine
	go startServer(server)

	// Wait for termination signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	// Cancel context to signal shutdown to all components
	cancel()
	logger.Get().Info().Msg("Graceful shutdown initiated")

	// Allow time for cleanup operations
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	// Perform any additional cleanup if needed
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Get().Error().Err(err).Msg("Server forced to shutdown")
	}

	logger.Get().Info().Msg("Server exited gracefully")
}

func formatMemory(mem interface{}) string {
	if mem == nil {
		return "N/A"
	}
	memFloat, ok := mem.(float64)
	if !ok {
		return "N/A"
	}

	memBytes := int64(memFloat)
	memMB := memBytes / 1024 / 1024

	if memMB > 1024 {
		memGB := float64(memBytes) / 1024 / 1024 / 1024
		return fmt.Sprintf("%.1f GB", memGB)
	}
	return fmt.Sprintf("%d MB", memMB)
}

func renderTemplate(w http.ResponseWriter, r *http.Request, name string, data map[string]interface{}) {
	// Inject authentication flag
	data["IsAuthenticated"] = sessionManager.GetBool(r.Context(), "authenticated")

	// Apply translations and other localization helpers
	localizePage(w, r, data)

	// If a template name is supplied, render it and store the result in the Content field.
	// When name is empty we assume the caller already populated data["Content"].
	if name != "" {
		buf := new(bytes.Buffer)
		if err := templates.ExecuteTemplate(buf, name, data); err != nil {
			logger.Get().Error().Err(err).Str("template", name).Msg("Error executing page template")
			http.Error(w, "Could not execute page template", http.StatusInternalServerError)
			return
		}
		data["Content"] = template.HTML(buf.String())
	}

	// Render the main layout which wraps whatever is present in data["Content"]
	if err := templates.ExecuteTemplate(w, "layout", data); err != nil {
		logger.Get().Error().Err(err).Msg("Error executing layout template")
		http.Error(w, "Could not execute layout template", http.StatusInternalServerError)
	}
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	logger.Get().Info().
		Str("handler", "indexHandler").
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("remote", r.RemoteAddr).
		Msg("Request received")
	logger.Get().Info().
		Str("path", r.URL.Path).
		Msg("Request received for index")
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	data := make(map[string]interface{})
	renderTemplate(w, r, "index.html", data)
}

func searchHandler(w http.ResponseWriter, r *http.Request) {
	logger.Get().Info().
		Str("handler", "searchHandler").
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("remote", r.RemoteAddr).
		Msg("Request received")
	logger.Get().Info().
		Str("path", r.URL.Path).
		Msg("Request received for search")
	data := make(map[string]interface{})

	if r.Method == http.MethodPost {
		r.ParseForm()
		vmid := r.FormValue("vmid")
		name := r.FormValue("name")

		// Validate VMID format on the backend
		if vmid != "" {
			match, _ := regexp.MatchString(`^[0-9]{1,10}$`, vmid)
			if !match {
				logger.Get().Warn().
					Str("vmid", vmid).
					Msg("Invalid VMID format received")
				data["Error"] = "Invalid VM ID: Please use 1 to 10 digits."
				renderTemplate(w, r, "search.html", data)
				return
			}
		}

		logger.Get().Info().
			Str("vmid", vmid).
			Str("name", name).
			Msg("Processing search request")

		// Use the global proxmox client
		if proxmoxClient == nil {
			logger.Get().Error().Msg("Proxmox client not initialized")
			http.Error(w, "Server configuration error", http.StatusInternalServerError)
			return
		}

		// Create context with timeout for API requests
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		// Fetch node names with context
		nodeNames, err := proxmox.GetNodeNamesWithContext(ctx, proxmoxClient)
		if err != nil {
			logger.Get().Error().Err(err).Msg("Failed to get node names")
			http.Error(w, "Failed to get node names", http.StatusInternalServerError)
			return
		}

		// Fetch node details with context
		nodesList := make([]map[string]interface{}, 0)
		for _, nodeName := range nodeNames {
			nodeDetails, err := proxmox.GetNodeDetailsWithContext(ctx, proxmoxClient, nodeName)
			if err != nil {
				logger.Get().Error().
					Err(err).
					Str("node", nodeName).
					Msg("Failed to get node details")
				continue
			}

			// Create a map for the Nodes template - convert int64 to float64 for template compatibility
			nodeMap := map[string]interface{}{
				"node":    nodeDetails.Node,
				"status":  "online",
				"cpu":     nodeDetails.CPU,
				"maxcpu":  float64(nodeDetails.MaxCPU),
				"mem":     float64(nodeDetails.Memory),
				"maxmem":  float64(nodeDetails.MaxMemory),
				"disk":    float64(nodeDetails.Disk),
				"maxdisk": float64(nodeDetails.MaxDisk),
			}
			nodesList = append(nodesList, nodeMap)
			logger.Get().Debug().Str("node", nodeName).Msg("Node details appended to list")
		}

		// Use the robust searchVM function to find matching VMs
		results, err := searchVM(proxmoxClient, vmid, name)
		if err != nil {
			logger.Get().Error().Err(err).Msg("Failed to execute VM search")
			http.Error(w, "Error searching for VMs", http.StatusInternalServerError)
			return
		}

		data["Results"] = results
		if vmid != "" {
			data["Query"] = vmid
		} else {
			data["Query"] = name
		}
	}

	renderTemplate(w, r, "search.html", data)
}

func createVmHandler(w http.ResponseWriter, r *http.Request) {
    err := loadSettings()
    if err != nil {
        http.Error(w, "Error loading settings: "+err.Error(), http.StatusInternalServerError)
        return
    }

    // Prepare template data with translations
    data := map[string]interface{}{
        "AvailableISOs":    appSettings.ISOs,
        "AvailableBridges": appSettings.VMBRs,
        "AvailableTags":    appSettings.Tags,
    }

    renderTemplate(w, r, "create_vm.html", data)
}

func adminHandler(w http.ResponseWriter, r *http.Request) {
	logger.Get().Info().
		Str("handler", "adminHandler").
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("remote", r.RemoteAddr).
		Msg("Request received")
	logger.Get().Info().
		Str("path", r.URL.Path).
		Msg("Request received for admin page")
	data := make(map[string]interface{})

	// Read settings first, to pass them to the template even if proxmox fails
	settings, err := readSettings()
	if err != nil {
		logger.Get().Error().
			Err(err).
			Msg("Failed to read settings")
		data["Error"] = "Failed to read application settings."
		renderTemplate(w, r, "admin.html", data)
		return
	}
	data["Settings"] = settings

	// Use the global proxmox client
	if proxmoxClient == nil {
		logger.Get().Error().Msg("Proxmox client not initialized")
		data["Error"] = "Server configuration error"
		renderTemplate(w, r, "admin.html", data)
		return
	}

	// Create context with timeout for API requests
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Fetch node names with context
	nodeNames, err := proxmox.GetNodeNamesWithContext(ctx, proxmoxClient)
	if err != nil {
		logger.Get().Error().
			Err(err).
			Msg("Failed to get node names")
		data["Error"] = "Failed to retrieve node list from Proxmox"
		renderTemplate(w, r, "admin.html", data)
		return
	}

	logger.Get().Info().
		Int("count", len(nodeNames)).
		Msg("Processing nodes for admin page")

	// Fetch node details with context
	nodeDetailsList := make([]proxmox.NodeDetails, 0)
	for _, nodeName := range nodeNames {
		nodeDetails, err := proxmox.GetNodeDetailsWithContext(ctx, proxmoxClient, nodeName)
		if err != nil {
			logger.Get().Error().
				Err(err).
				Str("node", nodeName).
				Msg("Failed to get node details")
			continue
		}
		// Use the concrete struct, not a pointer
		nodeDetailsList = append(nodeDetailsList, *nodeDetails)
	}
	data["NodeDetails"] = nodeDetailsList

	// Also fetch other necessary data for the admin page
	storagesResult, err := proxmox.GetStorages(proxmoxClient)
	if err != nil {
		logger.Get().Error().
			Err(err).
			Msg("Failed to get storages")
	}
	data["Storages"] = storagesResult

	// Localize the page (rendering will happen at the end of the function)
	localizePage(w, r, data)

	// Fetch all ISOs from all storages on all nodes
	allISOs := make([]map[string]interface{}, 0)
	if storagesMap, ok := storagesResult.(map[string]interface{}); ok {
		if storagesData, ok := storagesMap["data"].([]interface{}); ok {
			for _, nodeName := range nodeNames {
				for _, storage := range storagesData {
					if storageMap, ok := storage.(map[string]interface{}); ok {
						if contentType, ok := storageMap["content"].(string); ok && strings.Contains(contentType, "iso") {
							storageName := storageMap["storage"].(string)
							// Create context with timeout for ISO API requests
							isoCtx, isoCancel := context.WithTimeout(r.Context(), 5*time.Second)
							defer isoCancel()
							isos, err := proxmox.GetISOListWithContext(isoCtx, proxmoxClient, nodeName, storageName)
							if err != nil {
								logger.Get().Error().
									Err(err).
									Str("node", nodeName).
									Str("storage", storageName).
									Msg("Failed to get ISOs")
								continue
							}
							if isoData, ok := isos["data"].([]interface{}); ok {
								for _, iso := range isoData {
									if isoMap, ok := iso.(map[string]interface{}); ok {
										allISOs = append(allISOs, isoMap)
									}
								}
							}
						}
					}
				}
			}
		}
	}
	data["ISOs"] = allISOs

	allVMBRs := make([]map[string]interface{}, 0)
	for _, nodeName := range nodeNames {
		// Create context with timeout for VMBR API requests
		vmbrCtx, vmbrCancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer vmbrCancel()
		vmbrs, err := proxmox.GetVMBRsWithContext(vmbrCtx, proxmoxClient, nodeName)
		if err != nil {
			logger.Get().Error().Err(err).Str("node", nodeName).Msg("Failed to get VMBRs")
			continue
		}
		if vmbrData, ok := vmbrs["data"].([]interface{}); ok {
			for _, vmbr := range vmbrData {
				if vmbrMap, ok := vmbr.(map[string]interface{}); ok {
					vmbrMap["node"] = nodeName
					allVMBRs = append(allVMBRs, vmbrMap)
				}
			}
		}
	}
	data["VMBRs"] = allVMBRs

	data["Tags"] = settings.Tags

	renderTemplate(w, r, "admin.html", data)
}

func storagePageHandler(w http.ResponseWriter, r *http.Request) {
	logger.Get().Info().Str("handler", "storagePageHandler").Str("method", r.Method).Str("path", r.URL.Path).Str("remote", r.RemoteAddr).Msg("Request received")
	logger.Get().Info().Str("path", r.URL.Path).Msg("Request received for storage page")
	data := make(map[string]interface{})
	renderTemplate(w, r, "storage.html", data)
}

func isoPageHandler(w http.ResponseWriter, r *http.Request) {
	logger.Get().Info().Str("handler", "isoPageHandler").Str("method", r.Method).Str("path", r.URL.Path).Str("remote", r.RemoteAddr).Msg("Request received")
	logger.Get().Info().Str("path", r.URL.Path).Msg("Request received for iso page")
	data := make(map[string]interface{})
	renderTemplate(w, r, "iso.html", data)
}

func vmbrPageHandler(w http.ResponseWriter, r *http.Request) {
	logger.Get().Info().Str("handler", "vmbrPageHandler").Str("method", r.Method).Str("path", r.URL.Path).Str("remote", r.RemoteAddr).Msg("Request received")
	logger.Get().Info().Str("path", r.URL.Path).Msg("Request received for vmbr page")
	data := make(map[string]interface{})
	renderTemplate(w, r, "vmbr.html", data)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	logger.Get().Info().Str("handler", "healthHandler").Str("method", r.Method).Str("path", r.URL.Path).Str("remote", r.RemoteAddr).Msg("Request received")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	logger.Get().Info().Str("handler", "loginHandler").Str("method", r.Method).Str("path", r.URL.Path).Str("remote", r.RemoteAddr).Msg("Request received")
	if r.Method == http.MethodPost {
		password := r.FormValue("password")
		adminPasswordHash := os.Getenv("ADMIN_PASSWORD_HASH")

		if adminPasswordHash == "" {
			logger.Get().Error().Msg("ADMIN_PASSWORD_HASH is not set")
			data := map[string]interface{}{"Error": "Server configuration error"}
			renderTemplate(w, r, "login.html", data)
			return
		}

		if err := bcrypt.CompareHashAndPassword([]byte(adminPasswordHash), []byte(password)); err == nil {
			sessionManager.Put(r.Context(), "authenticated", true)
			http.Redirect(w, r, "/admin", http.StatusFound)
			return
		} else {
			logger.Get().Warn().Msg("Failed login attempt")
			data := map[string]interface{}{"Error": "Invalid password"}
			renderTemplate(w, r, "login.html", data)
			return
		}
	}

	renderTemplate(w, r, "login.html", make(map[string]interface{}))
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	logger.Get().Info().Str("handler", "logoutHandler").Str("method", r.Method).Str("path", r.URL.Path).Str("remote", r.RemoteAddr).Msg("Request received")
	sessionManager.Destroy(r.Context())
	http.Redirect(w, r, "/login", http.StatusFound)
}

func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !sessionManager.GetBool(r.Context(), "authenticated") {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		next.ServeHTTP(w, r)
	})
}
