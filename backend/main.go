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

	"pvmss/handlers"
	"pvmss/i18n"
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
		Addr: ":" + port,
		// Security-related timeouts
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
		Handler:      handlers.InitHandlers(stateManager),
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
	// Forcer le niveau de log en DEBUG pour le diagnostic
	logger.Init("debug")
}

func initializeApp(stateManager state.StateManager) error {
	// 1. Load settings first, as they are needed by the state manager.
	settings, modified, err := state.LoadSettings()
	if err != nil {
		return fmt.Errorf("failed to load settings: %w", err)
	}

	// 3. Initialize individual components.
	proxmoxClient, err := initProxmoxClient()
	if err != nil {
		return fmt.Errorf("failed to initialize Proxmox client: %w", err)
	}

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

	// 6. Initialize i18n, which depends on the logger.
	i18n.InitI18n()

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
	// Create base template with registered helper functions
	tmpl := template.New("").Funcs(templates.GetBaseFuncMap())

	// Get template directory path
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return nil, fmt.Errorf("could not get current file path")
	}

	rootDir := filepath.Dir(filepath.Dir(filename))
	frontendPath := filepath.Join(rootDir, "frontend")

	// Save the path for global use (e.g., serving static files)
	state.SetTemplatesPath(frontendPath)

	// Parse all HTML files in the frontend directory
	var templateFiles []string
	err := filepath.Walk(frontendPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".html") {
			relPath, _ := filepath.Rel(frontendPath, path)
			templateFiles = append(templateFiles, relPath)
			_, err = tmpl.ParseFiles(path)
			if err != nil {
				logger.Get().Error().
					Err(err).
					Str("template", relPath).
					Msg("Erreur lors du chargement du template")
			} else {
				logger.Get().Debug().
					Str("template", relPath).
					Msg("Template chargé avec succès")
			}
			return err
		}
		return nil
	})

	// Log des templates chargés
	logger.Get().Info().
		Strs("templates", templateFiles).
		Int("count", len(templateFiles)).
		Msg("Templates chargés")

	if err != nil {
		return nil, fmt.Errorf("error parsing templates: %w", err)
	}

	return tmpl, nil
}
