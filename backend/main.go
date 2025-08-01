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

	"github.com/alexedwards/scs/v2"
	"github.com/joho/godotenv"

	"pvmss/handlers"
	"pvmss/i18n"
	"pvmss/logger"
	"pvmss/proxmox"
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

	sessionManager := scs.New()
	sessionManager.Lifetime = 24 * time.Hour
	sessionManager.Cookie.Name = "pvmss_session"
	sessionManager.Cookie.HttpOnly = true
	sessionManager.Cookie.Secure = false // Set to true in production
	sessionManager.Cookie.SameSite = http.SameSiteLaxMode

	// 4. Populate the state manager with all components.
	if err := stateManager.SetProxmoxClient(proxmoxClient); err != nil {
		return fmt.Errorf("failed to set Proxmox client: %w", err)
	}
	if err := stateManager.SetTemplates(templates); err != nil {
		return fmt.Errorf("failed to set templates: %w", err)
	}
	if err := stateManager.SetSessionManager(sessionManager); err != nil {
		return fmt.Errorf("failed to set session manager: %w", err)
	}

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
