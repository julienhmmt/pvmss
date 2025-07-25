package main

import (
	"context"
	"fmt"
	"html/template"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/joho/godotenv"

	"pvmss/logger"
	"pvmss/proxmox"
	"pvmss/server"
	"pvmss/state"
	"pvmss/templates"
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



// loadSettings reads the application configuration from the `settings.json` file
// and stores it in the state package.
func loadSettings() error {
	settings, err := readSettings()
	if err != nil {
		return fmt.Errorf("error reading settings: %w", err)
	}
	state.SetAppSettings(settings)
	return nil
}

// initTemplates discovers, parses, and prepares all HTML templates for rendering.
// It uses the templates package for function definitions.
func initTemplates() (*template.Template, error) {
	// Get function map from templates package
	funcMap := templates.GetFuncMap()

	// Get the absolute path to the frontend directory
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return nil, fmt.Errorf("could not get current file path")
	}
	
	// Go up one directory from the current file location
	rootDir := filepath.Dir(filepath.Dir(filename))
	frontendPath := filepath.Join(rootDir, "frontend")
	templatePattern := filepath.Join(frontendPath, "*.html")

	logger.Get().Debug().
		Str("path", templatePattern).
		Msg("Loading templates from directory")

	tmpl, err := template.New("").Funcs(funcMap).ParseGlob(templatePattern)
	if err != nil {
		return nil, fmt.Errorf("error parsing templates: %w (path: %s)", err, templatePattern)
	}

	// Log loaded templates for debugging
	var templateNames []string
	for _, t := range tmpl.Templates() {
		templateNames = append(templateNames, t.Name())
	}
	logger.Get().Info().
		Strs("templates", templateNames).
		Msg("Successfully loaded templates")

	return tmpl, nil
}

// main is the application's entry point.
// It orchestrates the startup sequence: logger, environment variables, Proxmox client,
// internationalization, templates, and session management. It then starts the HTTP server
// and listens for OS signals to perform a graceful shutdown.
func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize logger first for proper error reporting
	initLogger()

	// Load environment variables
	if err := godotenv.Load("../.env"); err != nil {
		logger.Get().Warn().Msg("Error loading .env file, relying on environment variables")
	}

	// Initialize Proxmox client
	proxmoxClient, err := initProxmoxClient()
	if err != nil {
		logger.Get().Fatal().Err(err).Msg("Failed to initialize Proxmox client")
	}
	state.SetProxmoxClient(proxmoxClient)

	// Load application settings
	if err := loadSettings(); err != nil {
		logger.Get().Fatal().Err(err).Msg("Failed to load application settings")
	}

	// Initialize i18n
	InitI18n()

	// Initialize templates
	tmpl, err := initTemplates()
	if err != nil {
		logger.Get().Fatal().Err(err).Msg("Failed to initialize templates")
	}
	state.SetTemplates(tmpl)

	// Initialize session manager
	sm := scs.New()
	sm.Lifetime = 24 * time.Hour
	state.SetSessionManager(sm)

	// Setup and start HTTP server
	config := server.DefaultConfig()
	srv := server.Setup(config)

	// Start server in a goroutine
	go server.Start(srv)

	// Wait for termination signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	// Cancel context to signal shutdown to all components
	cancel()

	// Shutdown server gracefully
	if err := server.Shutdown(ctx, srv); err != nil {
		logger.Get().Error().Err(err).Msg("Server shutdown error")
	}
}
