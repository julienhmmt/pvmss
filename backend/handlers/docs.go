package handlers

import (
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"pvmss/components"
	"pvmss/i18n"
	"pvmss/logger"
	"pvmss/security"

	"github.com/gomarkdown/markdown"
	"github.com/julienschmidt/httprouter"
)

// CachedDoc holds a cached rendered documentation page
type CachedDoc struct {
	HTML template.HTML
	Lang string
}

// DocsHandler handles documentation routes with caching
type DocsHandler struct {
	docsDir string
	cache   map[string]*CachedDoc // key: "docType.lang"
	mu      sync.RWMutex
}

// MakeDocsHandler creates a new instance of DocsHandler
func MakeDocsHandler() *DocsHandler {
	log := logger.Get()

	docsDir, err := findDocsDir()
	if err != nil {
		log.Error().Err(err).Msg("Failed to find documentation directory")
	} else {
		log.Info().Str("docs_dir", docsDir).Msg("Found documentation directory")
	}

	return &DocsHandler{
		docsDir: docsDir,
		cache:   make(map[string]*CachedDoc),
	}
}

// DocsHandler handles requests for documentation with caching
func (h *DocsHandler) DocsHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	log := CreateHandlerLogger("DocsHandler", r)

	// Check if the documentation directory is available
	if h.docsDir == "" {
		log.Error().Msg("Documentation directory not configured")
		http.Error(w, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.ServerConfigError"), http.StatusServiceUnavailable)
		return
	}

	// Get language from query or use i18n detection
	lang := r.URL.Query().Get("lang")
	if lang == "" {
		lang = i18n.GetLanguage(r)
	}
	// Sanitize language code (security)
	lang = sanitizeLangCode(lang)

	// Determine the documentation type (admin, user or proxmox-permissions)
	docType := ps.ByName("type")
	if docType == "" {
		docType = "user"
	}
	// Sanitize doc type (security)
	if docType != "user" && docType != "admin" && docType != "proxmox-permissions" {
		log.Warn().Str("invalid_type", docType).Msg("Invalid doc type, using 'user'")
		docType = "user"
	}

	// Restrict Proxmox permissions documentation to admin users only
	if docType == "proxmox-permissions" && !IsAdmin(r) {
		log.Warn().Msg("Unauthorized access attempt to proxmox-permissions documentation")
		http.Error(w, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.Forbidden"), http.StatusForbidden)
		return
	}

	// Check cache first
	cacheKey := fmt.Sprintf("%s.%s", docType, lang)
	h.mu.RLock()
	cached, found := h.cache[cacheKey]
	h.mu.RUnlock()

	if found {
		log.Debug().
			Str("cache_key", cacheKey).
			Str("component", "docs").
			Str("operation", "serve_cached_docs").
			Str("reason", "cache_hit").
			Msg("Serving cached documentation")
		if cached != nil {
			h.renderDocsPage(w, r, cached.HTML, docType, cached.Lang)
			log.Info().Str("type", docType).Str("lang", lang).Msg("Served cached documentation")
			return
		}
	}

	// Cache miss - load and convert documentation
	docFile, finalLang := h.findDocFile(docType, lang)
	if docFile == "" {
		log.Warn().Str("type", docType).Str("lang", lang).Msg("Documentation not found")
		http.Error(w, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.NotFound"), http.StatusNotFound)
		return
	}

	// Read and convert markdown
	// Validate docFile path to prevent directory traversal
	absDocFile, err := filepath.Abs(docFile)
	if err != nil {
		log.Error().Err(err).Str("file", docFile).Msg("Invalid documentation file path")
		http.Error(w, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.InternalServer"), http.StatusInternalServerError)
		return
	}

	// Ensure the resolved file is under the configured docs directory
	absDocsDir, err := filepath.Abs(h.docsDir)
	if err != nil {
		log.Error().Err(err).Str("docs_dir", h.docsDir).Msg("Invalid docs directory path")
		http.Error(w, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.InternalServer"), http.StatusInternalServerError)
		return
	}
	cleanedFile := filepath.Clean(absDocFile)
	cleanedDir := filepath.Clean(absDocsDir)
	if cleanedFile != cleanedDir && !strings.HasPrefix(cleanedFile+string(os.PathSeparator), cleanedDir+string(os.PathSeparator)) {
		log.Warn().Str("file", cleanedFile).Str("docs_dir", cleanedDir).Msg("Attempt to access file outside docs directory")
		http.Error(w, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.NotFound"), http.StatusNotFound)
		return
	}

	content, err := os.ReadFile(absDocFile) // #nosec G304 - absDocFile validated and confined under docs directory
	if err != nil {
		log.Error().Err(err).Str("file", absDocFile).Msg("Failed to read documentation")
		http.Error(w, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.InternalServer"), http.StatusInternalServerError)
		return
	}

	// Convert Markdown to HTML
	htmlContent := template.HTML(markdown.ToHTML(content, nil, nil)) // #nosec G203 - Source markdown files are trusted and come from local docs directory

	// Store in cache
	h.mu.Lock()
	h.cache[cacheKey] = &CachedDoc{
		HTML: htmlContent,
		Lang: finalLang,
	}
	h.mu.Unlock()

	log.Debug().
		Str("cache_key", cacheKey).
		Str("component", "docs").
		Str("operation", "cache_documentation").
		Str("reason", "cache_updated").
		Msg("Documentation cached")

	h.renderDocsPage(w, r, htmlContent, docType, finalLang)
	log.Info().Str("type", docType).Str("lang", finalLang).Msg("Served documentation")
}

// findDocFile finds the documentation file with language fallback
func (h *DocsHandler) findDocFile(docType, lang string) (string, string) {
	// Try requested language first
	docFile := filepath.Join(h.docsDir, fmt.Sprintf("%s.%s.md", docType, lang))
	absDocsDir, err := filepath.Abs(h.docsDir)
	if err == nil {
		absDocFile, err := filepath.Abs(docFile)
		if err == nil && strings.HasPrefix(absDocFile, absDocsDir) {
			if _, err := os.Stat(absDocFile); err == nil {
				return absDocFile, lang
			}
		}
	}

	// Fallback to English
	if lang != "en" {
		docFile = filepath.Join(h.docsDir, fmt.Sprintf("%s.en.md", docType))
		absDocFile, err := filepath.Abs(docFile)
		if err == nil && strings.HasPrefix(absDocFile, absDocsDir) {
			if _, err := os.Stat(absDocFile); err == nil {
				return absDocFile, "en"
			}
		}
	}

	return "", ""
}

// sanitizeLangCode ensures the language code is safe (2-letter code only)
func sanitizeLangCode(lang string) string {
	lang = strings.ToLower(strings.TrimSpace(lang))
	if len(lang) != 2 || !isAlpha(lang) {
		return "en"
	}
	return lang
}

// isAlpha checks if string contains only letters
func isAlpha(s string) bool {
	for _, r := range s {
		if r < 'a' || r > 'z' {
			return false
		}
	}
	return true
}

// findDocsDir searches for the documentation directory
func findDocsDir() (string, error) {
	// Try multiple locations for the docs directory
	possiblePaths := []string{
		"/app/backend/docs", // Docker container absolute path
		"./docs",            // Current directory
		"../docs",           // Parent directory
		"./backend/docs",    // From project root
	}

	// Add runtime.Caller path as fallback
	if _, filename, _, ok := runtime.Caller(0); ok {
		handlersDir := filepath.Dir(filename)
		backendDir := filepath.Dir(handlersDir)
		possiblePaths = append(possiblePaths, filepath.Join(backendDir, "docs"))
	}

	for _, docsPath := range possiblePaths {
		absPath, err := filepath.Abs(docsPath)
		if err != nil {
			continue
		}
		if _, err := os.Stat(absPath); err == nil {
			return absPath, nil
		}
	}

	return "", fmt.Errorf("docs directory not found in any of the expected locations")
}

// renderDocsPage renders the documentation page using Templ
func (h *DocsHandler) renderDocsPage(w http.ResponseWriter, r *http.Request, htmlContent template.HTML, docType, lang string) {
	log := logger.Get()

	// Get username and admin status from session
	username := ""
	isAdmin := false
	if sessionManager := security.GetSession(r); sessionManager != nil {
		if user, ok := sessionManager.Get(r.Context(), "username").(string); ok {
			username = user
		}
		if admin, ok := sessionManager.Get(r.Context(), "is_admin").(bool); ok {
			isAdmin = admin
		}
	}

	// Get CSRF token
	csrfToken := ""
	if token, ok := security.CSRFTokenFromContext(r.Context()); ok {
		csrfToken = token
	}

	// Prepare docs data
	docsData := components.DocsData{
		Content:     string(htmlContent),
		CurrentLang: lang,
		DocType:     docType,
		Username:    username,
		Lang:        i18n.GetLanguage(r),
		CSRFToken:   csrfToken,
		IsAdmin:     isAdmin,
	}

	// Translation function wrapper
	translateFunc := func(key string) string {
		return i18n.Localize(i18n.GetLocalizerFromRequest(r), key)
	}

	// Render with Templ
	if err := components.DocsPage(docsData, translateFunc).Render(r.Context(), w); err != nil {
		log.Error().Err(err).Msg("Failed to render documentation page")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// RegisterRoutes registers documentation routes
func (h *DocsHandler) RegisterRoutes(router *httprouter.Router) {
	if router == nil {
		logger.Get().Error().Msg("Router is nil, cannot register documentation routes")
		return
	}

	// Route for user and admin documentation
	router.GET("/docs/:type", h.DocsHandler)

	// Alias for user documentation (default)
	router.GET("/docs", func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		h.DocsHandler(w, r, httprouter.Params{{Key: "type", Value: "user"}})
	})

	logger.Get().Info().Msg("Documentation routes registered: /docs, /docs/:type")
}
