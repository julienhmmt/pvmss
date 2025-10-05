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
	stateManager := state.NewAppState()

	initLogger()
	logger.Get().Info().Msg("Starting PVMSS")

	if err := godotenv.Load("../.env"); err != nil {
		logger.Get().Warn().Msg("No .env file found, using environment variables")
	}

	if err := initializeApp(stateManager); err != nil {
		logger.Get().Fatal().Err(err).Msg("Failed to initialize application")
	}

	sessionManager, err := security.InitSecurity()
	if err != nil {
		logger.Get().Fatal().Err(err).Msg("Failed to initialize security")
	}

	if err := stateManager.SetSessionManager(sessionManager); err != nil {
		logger.Get().Fatal().Err(err).Msg("Failed to set session manager")
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
		ReadHeaderTimeout: 5 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	go func() {
		logger.Get().Info().Str("port", port).Msg("Server starting...")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Get().Fatal().Err(err).Msg("Server failed to start")
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	logger.Get().Info().Msg("Shutdown signal received")

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

func initializeApp(stateManager state.StateManager) error {
	settings, modified, err := state.LoadSettings()
	if err != nil {
		return fmt.Errorf("failed to load settings: %w", err)
	}

	proxmoxClient, err := initProxmoxClient()
	if err != nil {
		return fmt.Errorf("failed to initialize Proxmox client: %w", err)
	}

	i18n.InitI18n()

	templates, err := initTemplates()
	if err != nil {
		return fmt.Errorf("failed to initialize templates: %w", err)
	}

	if err := stateManager.SetProxmoxClient(proxmoxClient); err != nil {
		return fmt.Errorf("failed to set Proxmox client: %w", err)
	}

	if connected := stateManager.CheckProxmoxConnection(); !connected {
		_, errorMsg := stateManager.GetProxmoxStatus()
		logger.Get().Warn().
			Str("error", errorMsg).
			Msg("Proxmox server not reachable, starting in read-only mode")
	}

	if err := stateManager.SetTemplates(templates); err != nil {
		return fmt.Errorf("failed to set templates: %w", err)
	}

	if modified {
		if err := stateManager.SetSettings(settings); err != nil {
			return fmt.Errorf("failed to save modified settings: %w", err)
		}
	} else {
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
		return nil, fmt.Errorf("missing required Proxmox environment variables: PROXMOX_URL, PROXMOX_API_TOKEN_NAME, PROXMOX_API_TOKEN_VALUE")
	}

	client, err := proxmox.NewClient(proxmoxURL, tokenID, tokenValue, insecureSkipVerify)
	if err != nil {
		return nil, fmt.Errorf("failed to create Proxmox client: %w", err)
	}

	client.SetTimeout(30 * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	nodes, err := proxmox.GetNodeNamesWithContext(ctx, client)
	if err != nil || len(nodes) == 0 {
		logger.Get().Error().Err(err).Msg("Failed to connect to Proxmox, starting in read-only mode")
		return client, nil
	}

	logger.Get().Info().
		Str("url", proxmoxURL).
		Str("token_id", tokenID).
		Bool("insecure", insecureSkipVerify).
		Strs("nodes", nodes).
		Msg("Proxmox client initialized")

	return client, nil
}

func initTemplates() (*template.Template, error) {
	funcMap := templates.GetBaseFuncMap()

	funcMap["T"] = func(messageID string, args ...interface{}) template.HTML {
		localizer := i18n.GetLocalizer(i18n.DefaultLang)
		localized := i18n.Localize(localizer, messageID)
		return template.HTML(localized)
	}

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return nil, fmt.Errorf("could not get current file path")
	}

	rootDir := filepath.Dir(filepath.Dir(filename))
	frontendPath := filepath.Join(rootDir, "frontend")

	templateFiles, err := templates.FindTemplateFiles(frontendPath)
	if err != nil {
		return nil, fmt.Errorf("error finding template files: %w", err)
	}

	tmpl, err := template.New("main").Funcs(funcMap).ParseFiles(templateFiles...)
	if err != nil {
		return nil, fmt.Errorf("error parsing templates: %w", err)
	}

	handlers.SetFrontendPath(frontendPath)

	var templateCount int
	for _, t := range tmpl.Templates() {
		if t.Name() != "" && strings.HasSuffix(t.Name(), ".html") {
			templateCount++
		}
	}

	logger.Get().Info().Int("count", templateCount).Msg("Templates loaded")

	return tmpl, nil
}
