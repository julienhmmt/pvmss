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

// CSS Context7 handler for serving CSS with context-aware optimizations
type CSSContext7Handler struct {
	basePath string
}

// NewCSSContext7Handler creates a new CSS context handler
func NewCSSContext7Handler(basePath string) *CSSContext7Handler {
	return &CSSContext7Handler{
		basePath: filepath.Join(basePath, "css"),
	}
}

// ServeCSSWithContext7 serves CSS files with context-aware optimizations
func (h *CSSContext7Handler) ServeCSSWithContext7(w http.ResponseWriter, r *http.Request) {
	// Extract the CSS file path
	cssPath := strings.TrimPrefix(r.URL.Path, "/css/")
	if cssPath == "" {
		http.NotFound(w, r)
		return
	}

	// Security check - prevent directory traversal
	if strings.Contains(cssPath, "..") || strings.HasPrefix(cssPath, "/") {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

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
	w.Header().Set("X-CSS-Context", "context7")
	w.Header().Set("X-Theme-Context", theme)

	// Serve different CSS variants based on context
	if err := h.serveContextualCSS(w, r, cssPath, theme); err != nil {
		logger.Get().Error().Err(err).Str("css_path", cssPath).Msg("Failed to serve CSS with context7")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

// serveContextualCSS serves CSS with contextual optimizations
func (h *CSSContext7Handler) serveContextualCSS(w http.ResponseWriter, r *http.Request, cssPath, theme string) error {
	fullPath := filepath.Join(h.basePath, cssPath)

	// Check if file exists
	if !fileExists(fullPath) {
		http.NotFound(w, r)
		return nil
	}

	// For bulma.min.css, we can potentially serve different variants
	if cssPath == "bulma.min.css" {
		return h.serveBulmaWithContext(w, r, fullPath, theme)
	}

	// For other CSS files, serve normally but with context headers
	http.ServeFile(w, r, fullPath)
	return nil
}

// serveBulmaWithContext serves Bulma CSS with theme context
func (h *CSSContext7Handler) serveBulmaWithContext(w http.ResponseWriter, r *http.Request, fullPath, theme string) error {
	// Add theme-specific CSS variables or modifications
	w.Header().Set("X-Bulma-Theme", theme)

	// Could potentially serve different Bulma variants here
	// For now, serve the standard bulma.min.css
	http.ServeFile(w, r, fullPath)

	logger.Get().Debug().
		Str("theme", theme).
		Str("css", "bulma.min.css").
		Msg("Served Bulma CSS with context7")

	return nil
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	// Additional security check - get absolute path
	abs, err := filepath.Abs(path)
	if err != nil || strings.Contains(abs, "..") {
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
						Msg("CSS served with context7")
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
