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
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/parser"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"

	"pvmss/i18n"
	"pvmss/logger"
	"pvmss/proxmox"
	"pvmss/state"
	"pvmss/templates"
)

func main() {
	// Initialize logger first
	initLogger()
	logger.Get().Info().Msg("Starting PVMSS")

	// Load environment variables
	if err := godotenv.Load("../.env"); err != nil {
		logger.Get().Warn().Msg("No .env file found, using environment variables")
	}

	// Initialize components
	if err := initializeApp(); err != nil {
		logger.Get().Fatal().Err(err).Msg("Failed to initialize application")
	}

	// Setup HTTP server
	mux := setupRoutes()
	port := os.Getenv("PORT")
	if port == "" {
		port = "50000"
	}

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server
	go func() {
		logger.Get().Info().Str("port", port).Msg("Server starting...")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Get().Fatal().Err(err).Msg("Server failed to start")
		}
	}()

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	logger.Get().Info().Msg("Shutdown signal received")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Get().Error().Err(err).Msg("Server shutdown error")
	} else {
		logger.Get().Info().Msg("Server shutdown complete")
	}
}

func initLogger() {
	level := os.Getenv("LOG_LEVEL")
	if level == "" {
		level = "info"
	}
	logger.Init(level)
}

func initializeApp() error {
	// Initialize the global state manager
	stateManager := state.InitGlobalState()

	// Initialize Proxmox client
	client, err := initProxmoxClient()
	if err != nil {
		return fmt.Errorf("failed to initialize Proxmox client: %w", err)
	}
	if err := stateManager.SetProxmoxClient(client); err != nil {
		return fmt.Errorf("failed to set Proxmox client: %w", err)
	}

	// Load settings
	settings, err := readSettings()
	if err != nil {
		return fmt.Errorf("failed to load settings: %w", err)
	}
	if err := stateManager.SetSettings(settings); err != nil {
		return fmt.Errorf("failed to set application settings: %w", err)
	}

	// Initialize templates
	tmpl, err := initTemplates()
	if err != nil {
		return fmt.Errorf("failed to initialize templates: %w", err)
	}
	if err := stateManager.SetTemplates(tmpl); err != nil {
		return fmt.Errorf("failed to set templates: %w", err)
	}

	// Initialize session manager
	sm := scs.New()
	sm.Lifetime = 24 * time.Hour
	if err := stateManager.SetSessionManager(sm); err != nil {
		return fmt.Errorf("failed to set session manager: %w", err)
	}

	// Initialize i18n
	i18n.InitI18n()

	logger.Get().Info().Msg("Application initialized successfully")
	return nil
}

func initProxmoxClient() (*proxmox.Client, error) {
	proxmoxURL := os.Getenv("PROXMOX_URL")
	proxmoxAPITokenName := os.Getenv("PROXMOX_API_TOKEN_NAME")
	proxmoxAPITokenValue := os.Getenv("PROXMOX_API_TOKEN_VALUE")
	insecureSkipVerify := os.Getenv("PROXMOX_VERIFY_SSL") == "false"

	if proxmoxURL == "" || proxmoxAPITokenName == "" || proxmoxAPITokenValue == "" {
		return nil, fmt.Errorf("missing required Proxmox configuration")
	}

	logger.Get().Info().
		Str("url", proxmoxURL).
		Bool("insecureSkipVerify", insecureSkipVerify).
		Msg("Initializing Proxmox client")

	return proxmox.NewClientWithOptions(
		proxmoxURL,
		proxmoxAPITokenName,
		proxmoxAPITokenValue,
		insecureSkipVerify,
	)
}

func initTemplates() (*template.Template, error) {
	// Create base template with functions
	tmpl := template.New("").Funcs(templates.GetBaseFuncMap())

	// Parse all HTML files in the frontend directory
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return nil, fmt.Errorf("could not get current file path")
	}

	rootDir := filepath.Dir(filepath.Dir(filename))
	frontendPath := filepath.Join(rootDir, "frontend")

	// Parse all HTML files in the frontend directory
	err := filepath.Walk(frontendPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".html") {
			_, err = tmpl.ParseFiles(path)
			return err
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("error parsing templates: %w", err)
	}

	return tmpl, nil
}

// IndexHandler handles the root path
func IndexHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// Initialize data with required i18n keys
	data := map[string]interface{}{
		"Title":           "PVMSS",
		"Description":     "Proxmox Virtual Machine Self-Service",
		"IsAuthenticated": isAuthenticated(r),
	}

	renderTemplate(w, r, "index", data)
}

func SearchHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		renderTemplate(w, r, "search", nil)
		return
	}

	if r.Method == http.MethodPost {
		vmid := r.FormValue("vmid")
		name := r.FormValue("name")

		logger.Get().Info().Str("vmid", vmid).Str("name", name).Msg("VM search")

		data := map[string]interface{}{
			"Results": []map[string]string{},
			"Query":   map[string]string{"vmid": vmid, "name": name},
		}
		renderTemplate(w, r, "search", data)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func LoginHandler(w http.ResponseWriter, r *http.Request) {
	if isAuthenticated(r) {
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
		return
	}

	if r.Method == http.MethodGet {
		renderTemplate(w, r, "login", nil)
		return
	}

	if r.Method == http.MethodPost {
		password := r.FormValue("password")
		storedHash := os.Getenv("ADMIN_PASSWORD_HASH")

		if storedHash == "" {
			logger.Get().Error().Msg("SECURITY ALERT: ADMIN_PASSWORD_HASH is not set.")
			data := map[string]interface{}{"Error": "Server configuration error."}
			renderTemplate(w, r, "login", data)
			return
		}

		err := bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(password))
		if err != nil {
			logger.Get().Warn().Msg("Failed login attempt")
			data := map[string]interface{}{"Error": "Invalid credentials"}
			renderTemplate(w, r, "login", data)
			return
		}

		stateManager := state.GetGlobalState()
		sessionManager := stateManager.GetSessionManager()
		sessionManager.Put(r.Context(), "authenticated", true)

		logger.Get().Info().Msg("Admin user logged in successfully")
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	stateManager := state.GetGlobalState()
	sessionManager := stateManager.GetSessionManager()
	_ = sessionManager.Destroy(r.Context())
	logger.Get().Info().Msg("User logged out")
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func AdminHandler(w http.ResponseWriter, r *http.Request) {
	settings := state.GetGlobalState().GetSettings()
	data := map[string]interface{}{"Settings": settings}
	renderTemplate(w, r, "admin", data)
}

func HealthHandler(w http.ResponseWriter, r *http.Request) {
	response := map[string]string{"status": "healthy", "app": "pvmss"}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Additional handlers needed by the application
func VMDetailsHandler(w http.ResponseWriter, r *http.Request) {
	vmid := r.URL.Query().Get("vmid")
	if vmid == "" {
		http.Error(w, "VM ID required", http.StatusBadRequest)
		return
	}

	logger.Get().Info().Str("vmid", vmid).Msg("VM details request")

	// TODO: Get actual VM details from Proxmox
	data := map[string]interface{}{
		"VM": map[string]string{
			"ID":     vmid,
			"Name":   "Sample VM",
			"Status": "running",
		},
	}

	renderTemplate(w, r, "vm_details", data)
}

func CreateVMHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		renderTemplate(w, r, "create_vm", nil)
		return
	}

	if r.Method == http.MethodPost {
		name := r.FormValue("name")
		logger.Get().Info().Str("name", name).Msg("VM creation request")

		data := map[string]interface{}{
			"Success": "VM creation initiated",
		}
		renderTemplate(w, r, "create_vm", data)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func StoragePageHandler(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Storages": []map[string]string{},
	}
	renderTemplate(w, r, "storage", data)
}

func TagsAPIHandler(w http.ResponseWriter, r *http.Request) {
	settings := state.GetGlobalState().GetSettings()
	tags := []string{}
	if settings != nil {
		tags = settings.Tags
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"tags": tags})
}

func StorageAPIHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: Get actual storage from Proxmox
	storages := []map[string]string{}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"storages": storages})
}

func findDocsDir() (string, error) {
	// Define possible documentation locations (in order of priority)
	possibleDirs := []string{
		// Development paths (relative to binary)
		filepath.Join(".", "docs"),
		filepath.Join(".", "backend", "docs"),
		
		// Docker paths
		filepath.Join("/app", "docs"),
		filepath.Join("/app", "backend", "docs"),
		
		// Fallback to current directory structure
		filepath.Dir("."),
	}

	// Get the absolute path to the executable
	exePath, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)
		possibleDirs = append(possibleDirs, 
			filepath.Join(exeDir, "docs"),
			filepath.Join(exeDir, "backend", "docs"),
		)
	}

	// Check each possible directory
	for _, dir := range possibleDirs {
		if dir == "" {
			continue
		}
		
		// Get absolute path and clean it
		dir, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		
		logger.Get().Debug().
			Str("path", dir).
			Msg("Checking for docs directory")
		
		// Check if directory exists and is readable
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			// Verify there are markdown files in the directory
			if files, err := filepath.Glob(filepath.Join(dir, "*.md")); err == nil && len(files) > 0 {
				// Verify we can read at least one file
				for _, file := range files {
					if _, err := os.Stat(file); err == nil {
						logger.Get().Info().
							Str("path", dir).
							Int("file_count", len(files)).
							Msg("Found valid docs directory")
						return dir, nil
					}
				}
			}
		}
	}
	
	// If we get here, no valid directory was found
	logger.Get().Error().
		Msg("Documentation directory not found in any expected location")
		
	// Try one last time with the current working directory
	if wd, err := os.Getwd(); err == nil {
		return wd, nil
	}
	
	return "", fmt.Errorf("documentation directory not found. Checked multiple locations")
}

func DocsHandler(w http.ResponseWriter, r *http.Request) {
	// Get language from URL or default to English
	lang := i18n.GetLanguage(r)
	
	// Determine document type from URL
	docType := "user"
	if strings.Contains(r.URL.Path, "admin") {
		docType = "admin"
	}

	logger.Get().Debug().
		Str("lang", lang).
		Str("docType", docType).
		Msg("Processing documentation request")

	// Find documentation directory
	docsDir, err := findDocsDir()
	if err != nil {
		logger.Get().Error().
			Err(err).
			Msg("Failed to locate documentation directory")
		http.Error(w, "Documentation system is not properly configured", http.StatusInternalServerError)
		return
	}

	// Try to load the requested language first, then fallback to English
	fileName := fmt.Sprintf("%s.%s.md", docType, lang)
	filePath := filepath.Join(docsDir, fileName)

	logger.Get().Debug().
		Str("file", filePath).
		Msg("Attempting to load documentation file")

	mdContent, err := os.ReadFile(filePath)
	if err != nil {
		// Fallback to English if the requested language is not available
		fallbackFile := fmt.Sprintf("%s.en.md", docType)
		fallbackPath := filepath.Join(docsDir, fallbackFile)
		
		logger.Get().Warn().
			Str("requested", fileName).
			Str("fallback", fallbackFile).
			Msg("Falling back to English documentation")
		
		mdContent, err = os.ReadFile(fallbackPath)
		if err != nil {
			logger.Get().Error().
				Err(err).
				Str("file", fallbackPath).
				Msg("Failed to read fallback documentation file")
			http.Error(w, "Documentation not available", http.StatusNotFound)
			return
		}
	}

	// Parse markdown to HTML
	extensions := parser.CommonExtensions | parser.AutoHeadingIDs
	mdParser := parser.NewWithExtensions(extensions)
	htmlContent := markdown.ToHTML(mdContent, mdParser, nil)

	// Prepare template data
	data := map[string]interface{}{
		"Content":        template.HTML(htmlContent),
		"Title":          fmt.Sprintf("%s Documentation", docType),
		"Description":    fmt.Sprintf("%s documentation for PVMSS", docType),
		"Lang":           lang,
		"LangEN":         strings.Replace(r.URL.Path, "/"+lang, "/en", 1),
		"LangFR":         strings.Replace(r.URL.Path, "/"+lang, "/fr", 1),
		"IsAdmin":        docType == "admin",
		"IsAuthenticated": isAuthenticated(r),
		"CurrentPath":    r.URL.Path,
	}

	// Set title in data map to be used by the template
	if docType == "admin" {
		data["Title"] = "Admin Documentation"
	} else {
		data["Title"] = "User Documentation"
	}

	renderTemplate(w, r, "docs", data)
}

// Routing setup
func setupRoutes() http.Handler {
	mux := http.NewServeMux()

	_, filename, _, _ := runtime.Caller(0)
	rootDir := filepath.Dir(filepath.Dir(filename))
	frontendPath := filepath.Join(rootDir, "frontend")

	cssPath := filepath.Join(frontendPath, "css")
	jsPath := filepath.Join(frontendPath, "js")

	logger.Get().Info().
		Str("css_path", cssPath).
		Str("js_path", jsPath).
		Msg("Serving static files from")

	// Serve static files with proper MIME types
	serveStatic := func(prefix, dir string) {
		fs := http.StripPrefix(prefix, http.FileServer(http.Dir(filepath.Join(frontendPath, dir))))
		mux.Handle(prefix, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Set proper cache control for static files
			w.Header().Set("Cache-Control", "public, max-age=31536000")

			// Set MIME types for known file extensions
			ext := filepath.Ext(r.URL.Path)
			switch ext {
			case ".css":
				w.Header().Set("Content-Type", "text/css; charset=utf-8")
			case ".js":
				w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
			case ".png":
				w.Header().Set("Content-Type", "image/png")
			case ".jpg", ".jpeg":
				w.Header().Set("Content-Type", "image/jpeg")
			case ".svg":
				w.Header().Set("Content-Type", "image/svg+xml")
			case ".ico":
				w.Header().Set("Content-Type", "image/x-icon")
			}

			fs.ServeHTTP(w, r)
		}))
	}

	// Register static file handlers
	serveStatic("/css/", "css")
	serveStatic("/js/", "js")

	// Routes
	mux.HandleFunc("/", IndexHandler)
	mux.HandleFunc("/login", LoginHandler)
	mux.HandleFunc("/logout", requireAuth(LogoutHandler)) // Protect logout route
	mux.HandleFunc("/search", SearchHandler)
	mux.HandleFunc("/admin", requireAuth(AdminHandler))
	mux.HandleFunc("/health", HealthHandler)
	mux.HandleFunc("/api/tags", requireAuth(TagsAPIHandler))
	mux.HandleFunc("/api/storage", requireAuth(StorageAPIHandler))
	mux.HandleFunc("/storage", requireAuth(StoragePageHandler))
	mux.HandleFunc("/vm/details", requireAuth(VMDetailsHandler))
	mux.HandleFunc("/create-vm", CreateVMHandler)
	mux.HandleFunc("/docs/", DocsHandler)

	stateManager := state.GetGlobalState()
	sessionManager := stateManager.GetSessionManager()
	return sessionManager.LoadAndSave(mux)
}

// Simple auth middleware
func requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isAuthenticated(r) {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		next.ServeHTTP(w, r)
	})
}

func isAuthenticated(r *http.Request) bool {
	stateManager := state.GetGlobalState()
	sessionManager := stateManager.GetSessionManager()
	return sessionManager.GetBool(r.Context(), "authenticated")
}

func renderTemplate(w http.ResponseWriter, r *http.Request, name string, data interface{}) {
	stateManager := state.GetGlobalState()
	tmpl := stateManager.GetTemplates()
	if tmpl == nil {
		logger.Get().Error().Msg("Templates not initialized")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Create a clone of the template to avoid modifying the original
	t, err := tmpl.Clone()
	if err != nil {
		logger.Get().Error().Err(err).Str("template", name).Msg("Failed to clone template")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Get the function map with request context and apply it to the template
	t = t.Funcs(templates.GetFuncMap(r))

	// Initialize data map
	dataMap := make(map[string]interface{})
	if data != nil {
		if dm, ok := data.(map[string]interface{}); ok {
			dataMap = dm
		} else {
			dataMap["Data"] = data
		}
	}

	// Add i18n data and common template variables
	i18n.LocalizePage(w, r, dataMap)
	dataMap["CurrentPath"] = r.URL.Path
	dataMap["IsHTTPS"] = r.TLS != nil
	dataMap["Host"] = r.Host

	// Set content type to HTML
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// First execute the content template into a buffer
	contentBuf := new(bytes.Buffer)
	if err := t.ExecuteTemplate(contentBuf, name, dataMap); err != nil {
		logger.Get().Error().
			Err(err).
			Str("template", name).
			Str("path", r.URL.Path).
			Msg("Template execution failed")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Add content to data map
	dataMap["Content"] = template.HTML(contentBuf.String())

	// Then execute the layout template with the content
	if err := t.ExecuteTemplate(w, "layout", dataMap); err != nil {
		logger.Get().Error().
			Err(err).
			Str("template", "layout").
			Str("path", r.URL.Path).
			Msg("Layout template execution failed")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}
