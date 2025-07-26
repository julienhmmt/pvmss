package main

import (
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

	"pvmss/handlers"
	"pvmss/i18n"
	"pvmss/logger"
	"pvmss/proxmox"
	"pvmss/state"
	"pvmss/templates"
)

func main() {

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
	router := handlers.InitHandlers()
	port := os.Getenv("PORT")
	if port == "" {
		port = "50000"
	}

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
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

// IndexHandler est un wrapper pour le handler d'index du package handlers
// Cette fonction est maintenue pour compatibilité mais devrait être remplacée
// par l'utilisation directe du handler du package handlers
func IndexHandler(w http.ResponseWriter, r *http.Request) {
	// Rediriger vers le handler d'index du package handlers
	handlers.IndexHandler(w, r)
}

func AdminHandler(w http.ResponseWriter, r *http.Request) {
	stateManager := state.GetGlobalState()
	client := stateManager.GetProxmoxClient()

	// Récupérer les paramètres
	settings := stateManager.GetSettings()

	// Récupérer les nœuds
	nodes, err := proxmox.GetNodeNames(client)
	if err != nil {
		logger.Get().Error().Err(err).Msg("Failed to get nodes")
		nodes = []string{} // Liste vide en cas d'erreur
	}

	// Récupérer les tags (à partir des paramètres ou autre source)
	tags := settings.Tags
	if tags == nil {
		tags = []string{}
	}

	// Prepare data for templates
	data := map[string]interface{}{
		"Settings": settings,
		"Nodes":    nodes,
		"Tags":     tags,
	}

	// Get node details for each node
	nodeDetailsList := make([]*proxmox.NodeDetails, 0)
	nodeDetailsMap := make(map[string]interface{})
	for _, node := range nodes {
		details, err := proxmox.GetNodeDetails(client, node)
		if err != nil {
			logger.Get().Error().Err(err).Str("node", node).Msg("Failed to get node details")
			continue
		}
		nodeDetailsList = append(nodeDetailsList, details)
		nodeDetailsMap[node] = details
	}
	data["NodeDetails"] = nodeDetailsList

	// Get storages
	storages, err := proxmox.GetStorages(client)
	if err != nil {
		logger.Get().Error().Err(err).Msg("Failed to get storages")
		storages = nil
	}
	data["Storages"] = storages

	// Get ISOs (for the first node if available)
	isoList := make([]interface{}, 0)
	if len(nodes) > 0 && storages != nil {
		// Convert storages to map to access data
		if storageMap, ok := storages.(map[string]interface{}); ok {
			if storageData, ok := storageMap["data"].([]interface{}); ok && len(storageData) > 0 {
				// Get first storage from the first node
				if storage, ok := storageData[0].(map[string]interface{}); ok {
					if storageName, ok := storage["storage"].(string); ok {
						// Get ISO list for this storage
						isos, err := proxmox.GetISOList(client, nodes[0], storageName)
						if err == nil {
							isoList = append(isoList, isos)
						} else {
							logger.Get().Error().Err(err).Str("node", nodes[0]).Str("storage", storageName).Msg("Failed to get ISO list")
						}
					}
				}
			}
		}
	}
	data["ISOs"] = isoList

	// Get VMBRs (for the first node if available)
	vmbrs := make(map[string]interface{})
	if len(nodes) > 0 {
		vmbrData, err := proxmox.GetVMBRs(client, nodes[0])
		if err == nil {
			vmbrs[nodes[0]] = vmbrData
		}
	}
	data["VMBRs"] = vmbrs

	renderTemplate(w, r, "admin", data)
}

func HealthHandler(w http.ResponseWriter, r *http.Request) {
	response := map[string]string{"status": "healthy", "app": "pvmss"}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
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
		"Content":         template.HTML(htmlContent),
		"Title":           fmt.Sprintf("%s Documentation", docType),
		"Description":     fmt.Sprintf("%s documentation for PVMSS", docType),
		"Lang":            lang,
		"LangEN":          strings.Replace(r.URL.Path, "/"+lang, "/en", 1),
		"LangFR":          strings.Replace(r.URL.Path, "/"+lang, "/fr", 1),
		"IsAdmin":         docType == "admin",
		"IsAuthenticated": isAuthenticated(r),
		"CurrentPath":     r.URL.Path,
	}

	// Set title in data map to be used by the template
	if docType == "admin" {
		data["Title"] = "Admin Documentation"
	} else {
		data["Title"] = "User Documentation"
	}

	renderTemplate(w, r, "docs", data)
}

// isAuthenticated est un wrapper pour la fonction isAuthenticated du package handlers
func isAuthenticated(r *http.Request) bool {
	return handlers.IsAuthenticated(r)
}

// renderTemplate est un wrapper pour la fonction renderTemplate du package handlers
func renderTemplate(w http.ResponseWriter, r *http.Request, name string, data interface{}) {
	// Utiliser la fonction renderTemplate du package handlers
	handlers.RenderTemplate(w, r, name, data)
}
