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
	// Initialiser le gestionnaire d'état global. C'est la première chose à faire.
	state.InitGlobalState()

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

	// Initialize security and get the session manager
	sessionManager, err := security.InitSecurity()
	if err != nil {
		logger.Get().Fatal().Err(err).Msg("Failed to initialize security")
	}

	// Store the session manager in the global state
	stateManager := state.GetGlobalState()
	if err := stateManager.SetSessionManager(sessionManager.SessionManager); err != nil {
		logger.Get().Fatal().Err(err).Msg("Failed to set session manager in state")
	}

	// Setup HTTP server (this now includes all middleware)
	handler := handlers.InitHandlers()

	port := os.Getenv("PORT")
	if port == "" {
		port = "50000"
	}

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: handler,
		// Security-related timeouts
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
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

func initializeApp() error {
	// 1. Load settings first, as they are needed by the state manager.
	settings, modified, err := state.LoadSettings()
	if err != nil {
		return fmt.Errorf("failed to load settings: %w", err)
	}

	// 2. Initialize the global state manager.
	// It's created empty and will be populated by subsequent steps.
	stateManager := state.InitGlobalState()

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
	// Get required configuration from environment variables
	proxmoxURL := os.Getenv("PROXMOX_URL")
	if proxmoxURL == "" {
		return nil, fmt.Errorf("missing PROXMOX_URL environment variable")
	}

	// Get API token credentials from environment variables
	tokenName := os.Getenv("PROXMOX_API_TOKEN_NAME")
	tokenValue := os.Getenv("PROXMOX_API_TOKEN_VALUE")

	// Fallback to username/password if token is not provided
	if tokenName == "" || tokenValue == "" {
		logger.Get().Warn().Msg("API token not provided, falling back to username/password")
		username := os.Getenv("PROXMOX_USER")
		password := os.Getenv("PROXMOX_PASSWORD")
		if username == "" || password == "" {
			return nil, fmt.Errorf("missing PROXMOX_API_TOKEN_NAME/TOKEN_VALUE or PROXMOX_USER/PASSWORD environment variables")
		}
		// Convert to token format (username@pve!tokenname=tokenvalue)
		tokenName = fmt.Sprintf("%s@pve!pvmss", username)
		tokenValue = password
	}

	// Parse insecure flag
	insecureSkipVerify := os.Getenv("PROXMOX_VERIFY_SSL") == "false"

	// Set timeout from environment or use default
	timeout := 30 * time.Second
	if timeoutStr := os.Getenv("PROXMOX_TIMEOUT"); timeoutStr != "" {
		if timeoutVal, err := time.ParseDuration(timeoutStr); err == nil {
			timeout = timeoutVal
		}
	}

	logger.Get().Info().
		Str("url", proxmoxURL).
		Bool("insecureSkipVerify", insecureSkipVerify).
		Dur("timeout", timeout).
		Msg("Initializing Proxmox client")

	// Create client with options
	client, err := proxmox.NewClientWithOptions(
		proxmoxURL,
		tokenName,
		tokenValue,
		insecureSkipVerify,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Proxmox client: %w", err)
	}

	// Set timeout
	client.Timeout = timeout

	// Verify connection with basic test request
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Try a simple API call to check connectivity
	_, err = client.GetWithContext(ctx, "/api2/json/version")
	if err != nil {
		logger.Get().Warn().
			Err(err).
			Str("url", proxmoxURL).
			Msg("Proxmox connection test failed")
		// Continue anyway as the server might be temporarily unavailable
	}

	return client, nil
}

func initTemplates() (*template.Template, error) {
	// Create base template with functions
	tmpl := template.New("").Funcs(templates.GetBaseFuncMap())

	// Get template directory path
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return nil, fmt.Errorf("could not get current file path")
	}

	rootDir := filepath.Dir(filepath.Dir(filename))
	frontendPath := filepath.Join(rootDir, "frontend")

	// Sauvegarder le chemin pour une utilisation globale (ex: servir les fichiers statiques)
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
