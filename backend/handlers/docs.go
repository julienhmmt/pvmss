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

	"pvmss/i18n"
	"pvmss/logger"

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

// NewDocsHandler creates a new instance of DocsHandler
func NewDocsHandler() *DocsHandler {
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
		http.Error(w, "Documentation not available", http.StatusServiceUnavailable)
		return
	}

	// Get language from query or use i18n detection
	lang := r.URL.Query().Get("lang")
	if lang == "" {
		lang = i18n.GetLanguage(r)
	}
	// Sanitize language code (security)
	lang = sanitizeLangCode(lang)

	// Determine the documentation type (admin or user)
	docType := ps.ByName("type")
	if docType == "" {
		docType = "user"
	}
	// Sanitize doc type (security)
	if docType != "user" && docType != "admin" {
		log.Warn().Str("invalid_type", docType).Msg("Invalid doc type, using 'user'")
		docType = "user"
	}

	// Check cache first
	cacheKey := fmt.Sprintf("%s.%s", docType, lang)
	h.mu.RLock()
	cached, found := h.cache[cacheKey]
	h.mu.RUnlock()

	if found {
		log.Debug().Str("cache_key", cacheKey).Msg("Serving cached documentation")
		// Serve from cache
		data := map[string]interface{}{
			"Title":       i18n.Localize(i18n.GetLocalizerFromRequest(r), "Docs.User.Title"),
			"Content":     cached.HTML,
			"CurrentLang": lang,
			"DocType":     docType,
		}
		renderTemplateInternal(w, r, "docs", data)
		log.Info().Str("type", docType).Str("lang", lang).Msg("Served cached documentation")
		return
	}

	// Cache miss - load and convert documentation
	docFile, finalLang := h.findDocFile(docType, lang)
	if docFile == "" {
		log.Warn().Str("type", docType).Str("lang", lang).Msg("Documentation not found")
		http.NotFound(w, r)
		return
	}

	// Read and convert markdown
	content, err := os.ReadFile(docFile)
	if err != nil {
		log.Error().Err(err).Str("file", docFile).Msg("Failed to read documentation")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Convert Markdown to HTML
	htmlContent := template.HTML(markdown.ToHTML(content, nil, nil))

	// Store in cache
	h.mu.Lock()
	h.cache[cacheKey] = &CachedDoc{
		HTML: htmlContent,
		Lang: finalLang,
	}
	h.mu.Unlock()

	log.Debug().Str("cache_key", cacheKey).Msg("Documentation cached")

	// Prepare data for the template
	data := map[string]interface{}{
		"Content":     htmlContent,
		"CurrentLang": finalLang,
		"DocType":     docType,
	}

	renderTemplateInternal(w, r, "docs", data)
	log.Info().Str("type", docType).Str("lang", finalLang).Msg("Served documentation")
}

// findDocFile finds the documentation file with language fallback
func (h *DocsHandler) findDocFile(docType, lang string) (string, string) {
	// Try requested language first
	docFile := filepath.Join(h.docsDir, fmt.Sprintf("%s.%s.md", docType, lang))
	if _, err := os.Stat(docFile); err == nil {
		return docFile, lang
	}

	// Fallback to English
	if lang != "en" {
		docFile = filepath.Join(h.docsDir, fmt.Sprintf("%s.en.md", docType))
		if _, err := os.Stat(docFile); err == nil {
			return docFile, "en"
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
	log := logger.Get()

	// Build list of possible locations
	possibleDirs := []string{
		"./docs",
		"../docs",
		"./backend/docs",
		"/app/backend/docs",
		"/app/docs",
	}

	// Add the directory of the running binary
	if execPath, err := os.Executable(); err == nil {
		possibleDirs = append(possibleDirs, filepath.Join(filepath.Dir(execPath), "docs"))
	}

	// Add the source file directory (call runtime.Caller only once)
	if _, filename, _, ok := runtime.Caller(0); ok {
		srcDir := filepath.Dir(filepath.Dir(filename))
		possibleDirs = append(possibleDirs, filepath.Join(srcDir, "docs"))
	}

	// Check each possible location
	for _, dir := range possibleDirs {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			// Verify directory contains files
			if entries, err := os.ReadDir(dir); err == nil && len(entries) > 0 {
				log.Info().Str("docs_dir", dir).Int("files", len(entries)).Msg("Found docs directory")
				return dir, nil
			}
		}
	}

	// If no valid directory was found
	return "", fmt.Errorf("documentation directory not found in: %v", possibleDirs)
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
