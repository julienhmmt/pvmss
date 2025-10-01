package handlers

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"pvmss/logger"
)

type themeContextKey string

const ThemeContextKey themeContextKey = "theme"

type CSSHandler struct {
	basePath string
}

// NewCSSHandler creates a new CSS handler
func NewCSSHandler(basePath string) *CSSHandler {
	return &CSSHandler{
		basePath: filepath.Join(basePath, "css"),
	}
}

func (h *CSSHandler) ServeCSS(w http.ResponseWriter, r *http.Request) {
	log := logger.Get().With().Str("handler", "CSSHandler.ServeCSS").Str("path", r.URL.Path).Logger()

	cssPath := strings.TrimPrefix(r.URL.Path, "/css/")
	if cssPath == "" {
		log.Debug().Msg("Empty CSS path, returning 404")
		http.NotFound(w, r)
		return
	}

	// Security check - prevent directory traversal
	if strings.Contains(cssPath, "..") || strings.HasPrefix(cssPath, "/") {
		log.Warn().Str("css_path", cssPath).Msg("Directory traversal attempt blocked")
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	log.Debug().Str("css_path", cssPath).Msg("Serving CSS file")

	// Get context information from request headers or query params
	theme := r.Header.Get("X-Theme")
	if theme == "" {
		theme = r.URL.Query().Get("theme")
	}
	if theme == "" {
		theme = "light" // default theme
	}

	// Set appropriate headers for CSS
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")

	// Add context-aware headers
	w.Header().Set("X-Theme-Context", theme)

	// Serve different CSS variants based on context
	if err := h.serveCSS(w, r, cssPath, theme); err != nil {
		logger.Get().Error().Err(err).Str("css_path", cssPath).Msg("Failed to serve CSS")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

func (h *CSSHandler) serveCSS(w http.ResponseWriter, r *http.Request, cssPath, theme string) error {
	log := logger.Get().With().Str("css_path", cssPath).Str("theme", theme).Logger()
	fullPath := filepath.Join(h.basePath, cssPath)

	// Check if file exists
	if !fileExists(fullPath) {
		log.Debug().Str("full_path", fullPath).Msg("CSS file not found")
		http.NotFound(w, r)
		return nil
	}

	log.Debug().Str("full_path", fullPath).Msg("CSS file found")

	// For bulma.min.css, we can potentially serve different variants
	if cssPath == "bulma.min.css" {
		return h.serveBulmaWithContext(w, r, fullPath, theme)
	}

	// For other CSS files, serve normally but with context headers
	log.Debug().Str("css_path", cssPath).Msg("Serving standard CSS file")
	http.ServeFile(w, r, fullPath)
	return nil
}

func (h *CSSHandler) serveBulmaWithContext(w http.ResponseWriter, r *http.Request, fullPath, theme string) error {
	w.Header().Set("X-Bulma-Theme", theme)
	// For now, serve the standard bulma.min.css
	http.ServeFile(w, r, fullPath)

	logger.Get().Debug().
		Str("theme", theme).
		Str("css", "bulma.min.css").
		Msg("Served Bulma CSS")

	return nil
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		logger.Get().Debug().Err(err).Str("path", path).Msg("File stat failed")
		return false
	}

	// Additional security check - get absolute path
	abs, err := filepath.Abs(path)
	if err != nil || strings.Contains(abs, "..") {
		if err != nil {
			logger.Get().Warn().Err(err).Str("path", path).Msg("Failed to get absolute path")
		} else {
			logger.Get().Warn().Str("path", path).Msg("Directory traversal detected in absolute path")
		}
		return false
	}

	return !info.IsDir()
}

// GetCSSContext7Middleware returns middleware for CSS context handling
func GetCSSContext7Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Add context information to the request
			ctx := r.Context()

			// Extract theme from various sources
			theme := extractThemeFromRequest(r)
			if theme != "" {
				ctx = context.WithValue(ctx, ThemeContextKey, theme)
				r = r.WithContext(ctx)
			}

			// Add performance timing
			start := time.Now()
			defer func() {
				duration := time.Since(start)
				if strings.HasPrefix(r.URL.Path, "/css/") {
					logger.Get().Debug().
						Str("path", r.URL.Path).
						Dur("duration", duration).
						Str("theme", theme).
						Msg("CSS served")
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// extractThemeFromRequest extracts theme information from the request
func extractThemeFromRequest(r *http.Request) string {
	// Try header first
	if theme := r.Header.Get("X-Theme"); theme != "" {
		return theme
	}

	// Try query parameter
	if theme := r.URL.Query().Get("theme"); theme != "" {
		return theme
	}

	// Try to extract from referrer or user agent
	userAgent := r.Header.Get("User-Agent")
	if strings.Contains(strings.ToLower(userAgent), "dark") {
		return "dark"
	}

	return "light" // default
}
