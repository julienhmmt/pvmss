package main

import (
	"context"
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

	"github.com/joho/godotenv"
	"github.com/nicksnyder/go-i18n/v2/i18n"

	"pvmss/handlers"
	custom_i18n "pvmss/i18n"
	"pvmss/logger"
	"pvmss/proxmox"
	"pvmss/security"
	"pvmss/state"
	"pvmss/templates"
)

func main() {
	// Initialize a dedicated app state (no global singleton)
	stateManager := state.NewAppState()

	initLogger()
	logger.Get().Info().Msg("Starting PVMSS")

	// Load environment variables
	if err := godotenv.Load("../.env"); err != nil {
		logger.Get().Warn().Msg("No .env file found, using environment variables")
	}

	// Initialize components
	if err := initializeApp(stateManager); err != nil {
		logger.Get().Fatal().Err(err).Msg("Failed to initialize application")
	}

	// Initialize security and get the session manager
	sessionManager, err := security.InitSecurity()
	if err != nil {
		logger.Get().Fatal().Err(err).Msg("Failed to initialize security")
	}

	// Store the session manager in the injected state.
	if err := stateManager.SetSessionManager(sessionManager); err != nil {
		logger.Get().Fatal().Err(err).Msg("Failed to set session manager on state")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "50000"
	}

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           handlers.InitHandlers(stateManager),
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		ReadHeaderTimeout: 5 * time.Second, // Prevent slow loris attacks
		MaxHeaderBytes:    1 << 20,         // 1MB max header size
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

// loadSettingsFromFile loads settings using the state package's LoadSettings function
// This wraps the deprecated function to maintain backward compatibility during migration
func loadSettingsFromFile() (*state.AppSettings, bool, error) {
	return state.LoadSettings()
}

func initializeApp(stateManager state.StateManager) error {
	// 1. Load settings first, as they are needed by the state manager.
	settings, modified, err := loadSettingsFromFile()
	if err != nil {
		return fmt.Errorf("failed to load settings: %w", err)
	}

	// 3. Initialize individual components.
	proxmoxClient, err := initProxmoxClient()
	if err != nil {
		return fmt.Errorf("failed to initialize Proxmox client: %w", err)
	}

	// 6. Initialize i18n, which depends on the logger.
	custom_i18n.InitI18n()

	templates, err := initTemplates()
	if err != nil {
		return fmt.Errorf("failed to initialize templates: %w", err)
	}

	// 4. Populate the state manager with all components.
	if err := stateManager.SetProxmoxClient(proxmoxClient); err != nil {
		return fmt.Errorf("failed to set Proxmox client: %w", err)
	}

	// Check Proxmox connection status
	if connected := stateManager.CheckProxmoxConnection(); !connected {
		// Log the error but don't fail the startup
		_, errorMsg := stateManager.GetProxmoxStatus()
		logger.Get().Warn().
			Str("error", errorMsg).
			Msg("Proxmox server is not reachable. The application will start in read-only mode.")
	}
	if err := stateManager.SetTemplates(templates); err != nil {
		return fmt.Errorf("failed to set templates: %w", err)
	}
	// Note: Session manager will be set later in main() after security initialization

	// 5. Handle settings: save if modified, otherwise just load into state.
	if modified {
		logger.Get().Info().Msg("Settings were modified during load, saving them back to the file.")
		if err := stateManager.SetSettings(settings); err != nil {
			return fmt.Errorf("failed to save modified settings: %w", err)
		}
	} else {
		logger.Get().Info().Msg("Settings loaded without modifications, updating state only.")
		stateManager.SetSettingsWithoutSave(settings)
	}

	return nil
}

func initProxmoxClient() (*proxmox.Client, error) {
	proxmoxURL := os.Getenv("PROXMOX_URL")
	tokenID := os.Getenv("PROXMOX_API_TOKEN_NAME")
	tokenValue := os.Getenv("PROXMOX_API_TOKEN_VALUE")
	insecureSkipVerify := os.Getenv("PROXMOX_VERIFY_SSL") == "false"

	if proxmoxURL == "" || tokenID == "" || tokenValue == "" {
		return nil, fmt.Errorf("missing Proxmox environment variables. PROXMOX_URL, PROXMOX_API_TOKEN_NAME, and PROXMOX_API_TOKEN_VALUE are required")
	}

	client, err := proxmox.NewClient(proxmoxURL, tokenID, tokenValue, insecureSkipVerify)
	if err != nil {
		return nil, fmt.Errorf("failed to create Proxmox client: %w", err)
	}

	client.SetTimeout(30 * time.Second)

	// Test the connection by getting node status
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	nodes, err := proxmox.GetNodeNamesWithContext(ctx, client)
	if err != nil || len(nodes) == 0 {
		logger.Get().Error().
			Err(err).
			Msg("Failed to connect to Proxmox server. The application will start in read-only mode.")

		// Return the client anyway, but log the error
		// The actual connection status will be handled by the state manager
		return client, nil
	}

	logger.Get().Info().
		Str("url", proxmoxURL).
		Str("token_id", tokenID).
		Bool("insecure", insecureSkipVerify).
		Strs("available_nodes", nodes).
		Msg("Proxmox client initialized successfully")

	return client, nil
}

func initTemplates() (*template.Template, error) {
	funcMap := templates.GetBaseFuncMap()
	bundle := custom_i18n.Bundle

	funcMap["T"] = func(messageID string, args ...interface{}) template.HTML {
		// The default language is used here, as we don't have a request context
		// at initialization time. The actual language will be determined at render time.
		localizeConfig := &i18n.LocalizeConfig{MessageID: messageID}
		localizer := i18n.NewLocalizer(bundle, bundle.LanguageTags()[0].String())
		localized, err := localizer.Localize(localizeConfig)
		if err != nil {
			// Fallback to message ID if translation fails
			return template.HTML(messageID)
		}
		return template.HTML(localized)
	}

	// Get template directory path
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return nil, fmt.Errorf("could not get current file path")
	}

	rootDir := filepath.Dir(filepath.Dir(filename))
	frontendPath := filepath.Join(rootDir, "frontend")

	// Parse all HTML files in the frontend directory
	var templateFiles []string
	templateFiles, err := templates.FindTemplateFiles(frontendPath)
	if err != nil {
		return nil, fmt.Errorf("error finding template files: %w", err)
	}

	tmpl, err := template.New("main").Funcs(funcMap).ParseFiles(templateFiles...)
	if err != nil {
		return nil, fmt.Errorf("error parsing templates: %w", err)
	}

	// Store the frontend path for later use by static file serving
	handlers.SetFrontendPath(frontendPath)

	// Log des templates chargés
	loadedTemplateNames := make([]string, 0, len(templateFiles))
	for _, t := range tmpl.Templates() {
		// We only want to log the base names of the files, not the full definition
		if t.Name() != "" && strings.HasSuffix(t.Name(), ".html") {
			loadedTemplateNames = append(loadedTemplateNames, t.Name())
		}
	}

	// Log des templates chargés
	logger.Get().Info().
		Strs("templates", templateFiles).
		Int("count", len(templateFiles)).
		Msg("Templates chargés")

	return tmpl, nil
}
