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
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"

	"pvmss/logger"
	"pvmss/proxmox"

	"github.com/nicksnyder/go-i18n/v2/i18n"
)

// Application-level globals for shared components.
// These are initialized once at startup and used throughout the application.
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

// initProxmoxClient creates and configures the Proxmox API client using environment variables.
// It returns an initialized client or an error if the configuration is invalid.
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

	// NewClientWithOptions will validate the required parameters
	client, err := proxmox.NewClientWithOptions(
		proxmoxURL,
		proxmoxAPITokenName,
		proxmoxAPITokenValue,
		insecureSkipVerify,
		// Add any additional options here if needed
		// proxmox.WithTimeout(30 * time.Second),
		// proxmox.WithCache(5 * time.Minute),
	)

	if err != nil {
		logger.Get().Error().Err(err).Msg("Failed to initialize Proxmox client")
		return nil, fmt.Errorf("failed to initialize Proxmox client: %w", err)
	}

	logger.Get().Info().Msg("Proxmox client initialized successfully")
	return client, nil
}

// Using AppSettings from settings.go instead of duplicate structure
var appSettings *AppSettings

// loadSettings reads the application configuration from the `settings.json` file
// and decodes it into the global `appSettings` variable.
func loadSettings() error {
	settings, err := readSettings()
	if err != nil {
		return fmt.Errorf("error reading settings: %w", err)
	}
	appSettings = settings
	return nil
}

// initTemplates discovers, parses, and prepares all HTML templates for rendering.
// It also registers a map of custom functions (funcMap) to be used within the templates.
func initTemplates() (*template.Template, error) {
	// Define template functions
	funcMap := template.FuncMap{
		// Conversion functions
		"int": func(v interface{}) int {
			switch v := v.(type) {
			case int:
				return v
			case int8:
				return int(v)
			case int16:
				return int(v)
			case int32:
				return int(v)
			case int64:
				return int(v)
			case uint:
				return int(v)
			case uint8:
				return int(v)
			case uint16:
				return int(v)
			case uint32:
				return int(v)
			case uint64:
				return int(v)
			case float32:
				return int(v)
			case float64:
				return int(v)
			case string:
				i, _ := strconv.Atoi(v)
				return i
			default:
				return 0
			}
		},
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
			logger.Get().Debug().Int("until_n", n).Msg("Calling 'until' function")
			var arr []int
			for i := 0; i < n; i++ {
				arr = append(arr, i)
			}
			return arr
		},
		"contains":  strings.Contains,
		"hasPrefix": strings.HasPrefix,
		"safe": func(s string) template.HTML {
			return template.HTML(s)
		},
		"nodeSocketsMax": func(node interface{}) int {
			// Default value if node is nil or doesn't have Sockets field
			if node == nil {
				return 1
			}

			// Use reflection to safely get the Sockets field
			nodeValue := reflect.ValueOf(node)
			if nodeValue.Kind() == reflect.Ptr {
				nodeValue = nodeValue.Elem()
			}

			if nodeValue.Kind() == reflect.Struct {
				socketsField := nodeValue.FieldByName("Sockets")
				if socketsField.IsValid() && socketsField.Kind() == reflect.Int {
					return int(socketsField.Int())
				}
			}

			return 8 // Default value if Sockets field not found
		},
		"nodeCoresMax": func(node interface{}) int {
			// Default value if node is nil or doesn't have Cores field
			if node == nil {
				return 1
			}

			// Use reflection to safely get the Cores field
			nodeValue := reflect.ValueOf(node)
			if nodeValue.Kind() == reflect.Ptr {
				nodeValue = nodeValue.Elem()
			}

			if nodeValue.Kind() == reflect.Struct {
				// Try to get MaxCPU first, then Cores
				coresField := nodeValue.FieldByName("MaxCPU")
				if !coresField.IsValid() {
					coresField = nodeValue.FieldByName("Cores")
				}

				if coresField.IsValid() {
					switch coresField.Kind() {
					case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
						return int(coresField.Int())
					case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
						return int(coresField.Uint())
					case reflect.Float32, reflect.Float64:
						return int(coresField.Float())
					}
				}
			}

			return 8 // Default value if Cores field not found
		},
		"nodeMemoryMaxGB": func(node interface{}) int64 {
			// Default value if node is nil or doesn't have memory field
			if node == nil {
				return 8
			}

			// Use reflection to safely get the memory field
			nodeValue := reflect.ValueOf(node)
			if nodeValue.Kind() == reflect.Ptr {
				nodeValue = nodeValue.Elem()
			}

			if nodeValue.Kind() == reflect.Struct {
				// Try different possible field names for memory
				memoryFields := []string{"MaxMem", "Memory", "MemTotal", "TotalMemory"}

				for _, fieldName := range memoryFields {
					memField := nodeValue.FieldByName(fieldName)
					if !memField.IsValid() {
						continue
					}

					// Convert to bytes if needed (assuming input might be in bytes, MB, etc.)
					var bytes int64
					switch memField.Kind() {
					case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
						bytes = memField.Int()
					case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
						bytes = int64(memField.Uint())
					case reflect.Float32, reflect.Float64:
						bytes = int64(memField.Float())
					default:
						continue
					}

					// Convert to GB (assuming input is in bytes)
					// If the value is suspiciously small, assume it's already in GB
					if bytes > 1024*1024*1024 { // > 1GB
						return bytes / (1024 * 1024 * 1024) // Convert bytes to GB
					}
					return bytes // Already in GB
				}
			}

			return 8 // Default value if memory field not found
		},
		"nodeDiskMaxGB": func(node interface{}) int64 {
			// Default value if node is nil or doesn't have disk field
			if node == nil {
				return 100 // Default to 100GB
			}

			// Use reflection to safely get the disk field
			nodeValue := reflect.ValueOf(node)
			if nodeValue.Kind() == reflect.Ptr {
				nodeValue = nodeValue.Elem()
			}

			if nodeValue.Kind() == reflect.Struct {
				// Try different possible field names for disk
				diskFields := []string{"MaxDisk", "Disk", "DiskTotal", "TotalDisk", "Storage"}

				for _, fieldName := range diskFields {
					diskField := nodeValue.FieldByName(fieldName)
					if !diskField.IsValid() {
						continue
					}

					// Convert to bytes if needed (assuming input might be in bytes, MB, etc.)
					var bytes int64
					switch diskField.Kind() {
					case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
						bytes = diskField.Int()
					case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
						bytes = int64(diskField.Uint())
					case reflect.Float32, reflect.Float64:
						bytes = int64(diskField.Float())
					default:
						continue
					}

					// Convert to GB (assuming input is in bytes)
					// If the value is suspiciously small, assume it's already in GB
					if bytes > 1024*1024*1024 { // > 1GB
						return bytes / (1024 * 1024 * 1024) // Convert bytes to GB
					}
					return bytes // Already in GB
				}
			}

			return 100 // Default value if disk field not found
		},
		"sub": func(a, b int) int { return a - b },
		"add": func(a, b int) int { return a + b },
		"sort": func(s []string) []string {
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

	// Log all registered functions for debugging
	logger.Get().Info().Msg("Registering template functions:")
	for name := range funcMap {
		logger.Get().Info().Str("function", name).Msg(" - Registered function")
	}

	// Create a new template with a dummy name first
	tmpl := template.New("")

	// Register all functions
	tmpl = tmpl.Funcs(funcMap)
	logger.Get().Info().Msg("Registered template functions")

	// Parse all HTML templates in the frontend directory
	templateDirs := []string{
		"/app/frontend", // For Docker container
		"frontend",      // For local development
		"../frontend",   // For local development (alternative)
	}

	// First, try to load layout.html explicitly
	layoutPaths := []string{
		"/app/frontend/layout.html",
		"frontend/layout.html",
		"../frontend/layout.html",
	}

	var layoutFile string
	for _, path := range layoutPaths {
		if _, err := os.Stat(path); err == nil {
			layoutFile = path
			break
		}
	}

	if layoutFile != "" {
		logger.Get().Info().Str("file", layoutFile).Msg("Parsing layout template")
		_, err := tmpl.ParseFiles(layoutFile)
		if err != nil {
			logger.Get().Error().Err(err).Str("file", layoutFile).Msg("Failed to parse layout template")
		} else {
			logger.Get().Info().Str("file", layoutFile).Msg("Successfully parsed layout template")
		}
	} else {
		logger.Get().Warn().Msg("Could not find layout.html, continuing without it")
	}

	// Now parse all other templates
	var templateErr error
	var foundTemplates bool

	for _, dir := range templateDirs {
		// Check if directory exists
		_, err := os.Stat(dir)
		if os.IsNotExist(err) {
			logger.Get().Debug().Str("dir", dir).Msg("Template directory does not exist")
			continue
		} else if err != nil {
			logger.Get().Error().Err(err).Str("dir", dir).Msg("Error checking template directory")
			continue
		}

		// Get all HTML files in the directory
		files, err := filepath.Glob(filepath.Join(dir, "*.html"))
		if err != nil {
			logger.Get().Error().Err(err).Str("dir", dir).Msg("Error listing template files")
			continue
		}

		if len(files) == 0 {
			logger.Get().Warn().Str("dir", dir).Msg("No template files found in directory")
			continue
		}

		logger.Get().Info().Str("dir", dir).Int("files", len(files)).Msg("Found template files")

		// Parse all files except layout.html (already parsed)
		var filesToParse []string
		for _, file := range files {
			if filepath.Base(file) != "layout.html" {
				filesToParse = append(filesToParse, file)
			}
		}

		if len(filesToParse) > 0 {
			logger.Get().Info().Strs("files", filesToParse).Msg("Parsing template files")
			_, templateErr = tmpl.ParseFiles(filesToParse...)
			if templateErr == nil {
				foundTemplates = true
				logger.Get().Info().Str("dir", dir).Msg("Successfully parsed templates from directory")
				break
			}

			logger.Get().
				Error().
				Err(templateErr).
				Str("templateDir", dir).
				Msg("Failed to parse templates from directory, trying next one")
		}
	}

	if !foundTemplates {
		return nil, fmt.Errorf("failed to parse any template files from directories: %v", templateDirs)
	}

	// Log all defined templates for debugging
	logger.Get().Info().Msg("Defined templates:")
	for _, t := range tmpl.Templates() {
		if t.Name() != "" {
			logger.Get().Info().Str("template", t.Name()).Msg(" - Defined template")
		}
	}

	// Store the parsed templates in the global variable
	templates = tmpl

	return tmpl, nil
}

// setupServer configures and returns a new HTTP server.
// It sets up the router, registers all application routes (including static files and API endpoints),
// and wraps the main handler with session management middleware and security headers.
func setupServer() *http.Server {
	// Get port from environment
	port := os.Getenv("PORT")
	if port == "" {
		port = "50000"
		logger.Get().Info().Str("port", port).Msg("Using default port")
	}

	// Create router
	r := http.NewServeMux()

	// Serve static files
	r.Handle("/css/", http.StripPrefix("/css/", http.FileServer(http.Dir("frontend/css"))))
	r.Handle("/js/", http.StripPrefix("/js/", http.FileServer(http.Dir("frontend/js"))))

	// Serve HTML files directly from the frontend directory
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Check if the request is for a specific file
		if r.URL.Path != "/" && r.URL.Path != "" && !strings.HasPrefix(r.URL.Path, "/api") {
			// Try to serve the file directly
			filePath := filepath.Join("frontend", r.URL.Path)
			if _, err := os.Stat(filePath); err == nil {
				http.ServeFile(w, r, filePath)
				return
			}
			// If file not found, serve index.html for SPA routing
			http.ServeFile(w, r, "frontend/index.html")
		} else {
			// Default to index handler
			indexHandler(w, r)
		}
	})

	// Handlers
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
	// Apply security middleware chain
	handler := securityHeadersMiddleware(sessionManager.LoadAndSave(csrfMiddleware(r)))
	
	return &http.Server{
		Addr:         ":" + port,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}

// startServer starts the provided HTTP server and logs a fatal error if it fails.
func startServer(srv *http.Server) {
	logger.Get().Info().Str("addr", srv.Addr).Msg("Starting server...")
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Get().Fatal().Err(err).Msg("Server failed to start")
	}
}

// main is the application's entry point.
// It orchestrates the startup sequence: logger, environment variables, Proxmox client,
// internationalization, templates, and session management. It then starts the HTTP server
// and listens for OS signals to perform a graceful shutdown.
func main() {
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize logger first for proper error reporting
	initLogger()

	// Load environment variables
	if err := godotenv.Load("../.env"); err != nil {
		logger.Get().Warn().Msg("Error loading .env file, relying on environment variables")
	}

	// Initialize Proxmox client
	var err error
	proxmoxClient, err = initProxmoxClient()
	if err != nil {
		logger.Get().Fatal().Err(err).Msg("Failed to initialize Proxmox client")
	}

	// Initialize i18n
	InitI18n()

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
	server := setupServer()

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

// formatMemory is a helper function to convert a memory value (in bytes) into a human-readable string (MB or GB).
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

// localize is a helper function to get a localized string for a given key.
// It has been superseded by the `safeLocalize` function in `i18n.go` for most uses to prevent panics.
func localize(r *http.Request, key string) string {
	lang := getLanguage(r)
	localizer := i18n.NewLocalizer(Bundle, lang)
	return localizer.MustLocalize(&i18n.LocalizeConfig{
		MessageID: key,
	})
}

// renderTemplate handles the rendering of HTML pages.
// It injects common data (like authentication status and language), localizes the page content,
// executes the specified page template, and then embeds the result into the main layout template.
func renderTemplate(w http.ResponseWriter, r *http.Request, name string, data map[string]interface{}) {
	// Inject authentication flag and language
	data["IsAuthenticated"] = sessionManager.GetBool(r.Context(), "authenticated")
	lang := getLanguage(r)
	data["Lang"] = lang
	data["LangEN"] = "/?lang=en"
	data["LangFR"] = "/?lang=fr"
	
	// Generate CSRF token for forms
	data["CSRFToken"] = generateCSRFToken()

	// Apply translations and other localization helpers
	localizePage(w, r, data)

	buf := new(bytes.Buffer)
	err := templates.ExecuteTemplate(buf, name, data)
	if err != nil {
		logger.Get().Error().Err(err).Str("template", name).Msg("Error executing page template")
		http.Error(w, "Could not execute page template", http.StatusInternalServerError)
		return
	}
	data["SafeContent"] = template.HTML(buf.String())

	// Render the main layout which wraps the content
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.ExecuteTemplate(w, "layout", data); err != nil {
		logger.Get().Error().Err(err).Msg("Error executing layout template")
		http.Error(w, "Could not execute layout template", http.StatusInternalServerError)
	}
}

// indexHandler handles requests to the root ("/") path.
// It serves the main landing page of the application.
func indexHandler(w http.ResponseWriter, r *http.Request) {
	logger.Get().Info().
		Str("handler", "indexHandler").
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("remote", r.RemoteAddr).
		Msg("Request received")

	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	data := make(map[string]interface{})
	data["Title"] = "PVMSS - " + localize(r, "Navbar.Home")

	renderTemplate(w, r, "index.html", data)
}

// searchHandler handles VM search requests.
// It processes POST requests containing search criteria (VMID or name),
// fetches results from the Proxmox API, and renders the search results page.
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
        vmid := validateInput(r.FormValue("vmid"), 10)
        name := validateInput(r.FormValue("name"), 100)

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
        data["Nodes"] = nodesList
        if vmid != "" {
            data["Query"] = vmid
        } else {
            data["Query"] = name
        }
    }

    renderTemplate(w, r, "search.html", data)
}

// createVmHandler handles the display of the VM creation page.
// It loads application settings to provide the template with necessary data like available ISOs, networks, and tags.
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

// adminHandler renders the main administration page.
// It fetches comprehensive data from both the Proxmox API (nodes, storage, ISOs, VMBRs)
// and the local settings.json file to populate the template with all necessary configuration options.
func adminHandler(w http.ResponseWriter, r *http.Request) {
    logger.Get().Info().
        Str("handler", "adminHandler").
        Str("method", r.Method).
        Str("path", r.URL.Path).
        Str("remote", r.RemoteAddr).
        Msg("Request received")

    data := make(map[string]interface{})

    // Use cached settings instead of reading from disk every time
    if appSettings == nil {
        logger.Get().Error().Msg("Global settings not initialized")
        data["Error"] = "Application settings not loaded."
        renderTemplate(w, r, "admin.html", data)
        return
    }
    data["Settings"] = appSettings
    settings := appSettings // Local alias for compatibility

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
    // For debugging: log all node MaxMemory values
    for _, nd := range nodeDetailsList {
        logger.Get().Info().Str("node", nd.Node).Float64("MaxMemory", nd.MaxMemory).Msg("Node RAM for limits page")
    }

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

// loginHandler handles user authentication.
// For GET requests, it displays the login page.
// For POST requests, it validates the password against the stored bcrypt hash,
// authenticates the session, and redirects to the admin page on success.
func loginHandler(w http.ResponseWriter, r *http.Request) {
	logger.Get().Info().Str("handler", "loginHandler").Str("method", r.Method).Str("path", r.URL.Path).Str("remote", r.RemoteAddr).Msg("Request received")
	if r.Method == http.MethodPost {
		// Check rate limiting
		clientIP := r.RemoteAddr
		if !checkRateLimit(clientIP) {
			logger.Get().Warn().Str("ip", clientIP).Msg("Rate limit exceeded for login attempts")
			data := map[string]interface{}{"Error": "Too many login attempts. Please try again later."}
			renderTemplate(w, r, "login.html", data)
			return
		}
		
		password := validateInput(r.FormValue("password"), 200)
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
			recordLoginAttempt(clientIP)
			logger.Get().Warn().Str("ip", clientIP).Msg("Failed login attempt")
			data := map[string]interface{}{"Error": "Invalid password"}
			renderTemplate(w, r, "login.html", data)
			return
		}
	}

	renderTemplate(w, r, "login.html", make(map[string]interface{}))
}

// logoutHandler handles user logout.
// It destroys the current session and redirects the user to the homepage.
func logoutHandler(w http.ResponseWriter, r *http.Request) {
	logger.Get().Info().Str("handler", "logoutHandler").Str("method", r.Method).Str("path", r.URL.Path).Str("remote", r.RemoteAddr).Msg("Request received")
	sessionManager.Destroy(r.Context())
	http.Redirect(w, r, "/login", http.StatusFound)
}

func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !sessionManager.GetBool(r.Context(), "authenticated") {
			// Si c'est une requête API, renvoyer une erreur 401 au lieu de rediriger
			if strings.HasPrefix(r.URL.Path, "/api/") {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"success": false,
					"error":   "Unauthorized",
				})
				return
			}
			// Pour les autres requêtes, rediriger vers la page de connexion
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		next.ServeHTTP(w, r)
	})
}
