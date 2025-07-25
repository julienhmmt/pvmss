// Package server provides HTTP server configuration and setup for the PVMSS application
package server

import (
	"context"
	"net/http"
	"os"
	"time"

	"pvmss/logger"
	"pvmss/security"
	"pvmss/state"
)

// Config holds server configuration
type Config struct {
	Port         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

// DefaultConfig returns default server configuration
func DefaultConfig() *Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "50000"
	}

	return &Config{
		Port:         port,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}

// Setup configures and returns a new HTTP server with all routes
func Setup(config *Config) *http.Server {
	r := http.NewServeMux()

	// Setup all routes
	setupStaticRoutes(r)
	setupAPIRoutes(r)
	setupPageRoutes(r)
	setupAuthRoutes(r)
	setupDocRoutes(r)
	setupHealthRoute(r)

	// Apply security middleware chain
	sm := state.GetSessionManager()
	handler := security.HeadersMiddleware(
		sm.LoadAndSave(
			security.CSRFMiddleware(r)))

	return &http.Server{
		Addr:         ":" + config.Port,
		Handler:      handler,
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
		IdleTimeout:  config.IdleTimeout,
	}
}

// Start starts the HTTP server
func Start(srv *http.Server) {
	logger.Get().Info().Str("addr", srv.Addr).Msg("Starting server...")
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Get().Fatal().Err(err).Msg("Server failed to start")
	}
}

// Shutdown gracefully shuts down the server
func Shutdown(ctx context.Context, srv *http.Server) error {
	logger.Get().Info().Msg("Graceful shutdown initiated")
	
	shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Get().Error().Err(err).Msg("Server forced to shutdown")
		return err
	}

	logger.Get().Info().Msg("Server shutdown complete")
	return nil
}
