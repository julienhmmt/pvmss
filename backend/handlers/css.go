package handlers

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"pvmss/logger"
)

// CSSHandler serves CSS files with security checks and optimized caching
type CSSHandler struct {
	basePath string
}

// NewCSSHandler creates a new CSS handler with the specified base path
func NewCSSHandler(basePath string) *CSSHandler {
	return &CSSHandler{
		basePath: filepath.Join(basePath, "css"),
	}
}

// ServeCSS serves CSS files with security checks, proper headers, and aggressive caching
func (h *CSSHandler) ServeCSS(w http.ResponseWriter, r *http.Request) {
	log := CreateHandlerLogger("CSSHandler.ServeCSS", r)

	// Extract CSS filename from URL path
	cssPath := strings.TrimPrefix(r.URL.Path, "/css/")
	if cssPath == "" {
		log.Debug().Msg("Empty CSS path, returning 404")
		http.NotFound(w, r)
		return
	}

	// Security: Prevent directory traversal attacks
	if strings.Contains(cssPath, "..") || strings.HasPrefix(cssPath, "/") {
		log.Warn().Str("css_path", cssPath).Msg("Directory traversal attempt blocked")
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Build full file path
	fullPath := filepath.Join(h.basePath, cssPath)

	// Security: Verify file exists and is not a directory
	if !h.isValidCSSFile(fullPath) {
		log.Debug().Str("full_path", fullPath).Msg("CSS file not found or invalid")
		http.NotFound(w, r)
		return
	}

	log.Debug().Str("css_path", cssPath).Msg("Serving CSS file")

	// Set optimal headers for CSS delivery
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	// Aggressive caching for CSS files (1 year) - they should be versioned
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	// Security header to prevent MIME type sniffing
	w.Header().Set("X-Content-Type-Options", "nosniff")

	// Serve the CSS file
	http.ServeFile(w, r, fullPath)
}

// isValidCSSFile checks if the path points to a valid CSS file (exists and is not a directory)
func (h *CSSHandler) isValidCSSFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Get().Debug().Err(err).Str("path", path).Msg("File stat failed")
		}
		return false
	}

	// Additional security: Ensure it's a file, not a directory
	if info.IsDir() {
		logger.Get().Warn().Str("path", path).Msg("Attempted to serve directory as CSS file")
		return false
	}

	// Additional security: Verify absolute path doesn't contain traversal
	abs, err := filepath.Abs(path)
	if err != nil {
		logger.Get().Warn().Err(err).Str("path", path).Msg("Failed to get absolute path")
		return false
	}

	// Ensure the resolved path is still within the base path
	absBase, _ := filepath.Abs(h.basePath)
	if !strings.HasPrefix(abs, absBase) {
		logger.Get().Warn().
			Str("requested_path", abs).
			Str("base_path", absBase).
			Msg("Path traversal attempt detected")
		return false
	}

	return true
}
