package apiv1

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
	"github.com/julienschmidt/httprouter"

	"pvmss/logger"
	"pvmss/state"
)

// DocsAPIHandler serves pre-rendered markdown documentation as HTML.
type DocsAPIHandler struct {
	docsDir string
	state   state.StateManager
	cache   map[string]string // key: "type.lang" → HTML string
	mu      sync.RWMutex
}

// MakeDocsAPIHandler creates a new DocsAPIHandler, resolving the docs directory.
func MakeDocsAPIHandler(s state.StateManager) *DocsAPIHandler {
	dir, err := findAPIDocsDir()
	if err != nil {
		logger.Get().Warn().Err(err).Msg("api/v1/docs: docs directory not found")
	}
	return &DocsAPIHandler{
		docsDir: dir,
		state:   s,
		cache:   make(map[string]string),
	}
}

// GetDoc handles GET /api/v1/docs/:type
// Returns { "html": "<rendered markdown>" } for the requested doc type and language.
// Allowed types: user, admin, proxmox-permissions, cloud-init.
// Language is read from the Accept-Language header or ?lang= param; falls back to "en".
func (h *DocsAPIHandler) GetDoc(w http.ResponseWriter, r *http.Request) {
	ps := httprouter.ParamsFromContext(r.Context())
	docType := ps.ByName("type")

	allowed := map[string]bool{
		"user":                true,
		"admin":               true,
		"proxmox-permissions": true,
		"cloud-init":          true,
	}
	if !allowed[docType] {
		errBadRequest(w, "invalid doc type")
		return
	}

	// admin and proxmox-permissions require a valid JWT.
	adminOnlyTypes := map[string]bool{"admin": true, "proxmox-permissions": true}
	if adminOnlyTypes[docType] {
		claims, ok := h.parseClaims(r)
		if !ok {
			errUnauthorized(w)
			return
		}
		if docType == "proxmox-permissions" && !claims.IsAdmin {
			errForbidden(w)
			return
		}
	}

	lang := sanitizeDocLang(r.URL.Query().Get("lang"))
	if lang == "" {
		lang = detectLang(r)
	}

	cacheKey := docType + "." + lang
	h.mu.RLock()
	cached, ok := h.cache[cacheKey]
	h.mu.RUnlock()
	if ok {
		writeJSON(w, map[string]string{"html": cached})
		return
	}

	htmlContent, usedLang, err := h.renderDoc(docType, lang)
	if err != nil {
		logger.Get().Warn().Err(err).Str("type", docType).Str("lang", lang).Msg("api/v1/docs: doc not found")
		writeError(w, http.StatusNotFound, "not_found", "Documentation not found")
		return
	}

	// Cache under both the requested key and the resolved key.
	h.mu.Lock()
	h.cache[cacheKey] = htmlContent
	h.cache[docType+"."+usedLang] = htmlContent
	h.mu.Unlock()

	writeJSON(w, map[string]string{"html": htmlContent})
}

// renderDoc reads the markdown file and converts it to an HTML string.
func (h *DocsAPIHandler) renderDoc(docType, lang string) (string, string, error) {
	file, usedLang := h.findFile(docType, lang)
	if file == "" {
		return "", "", os.ErrNotExist
	}
	content, err := os.ReadFile(file) // #nosec G304 — path validated in findFile
	if err != nil {
		return "", "", err
	}

	extensions := parser.CommonExtensions | parser.AutoHeadingIDs
	p := parser.NewWithExtensions(extensions)
	// HrefTargetBlank is intentionally omitted: it would apply target="_blank" to
	// internal anchor links (#section) as well, breaking in-page navigation.
	// External links get target="_blank" via post-processing below.
	opts := html.RendererOptions{Flags: html.CommonFlags}
	renderer := html.NewRenderer(opts)
	rendered := markdown.ToHTML(content, p, renderer)

	// Add target="_blank" rel="noopener noreferrer" to external links only.
	out := addExternalLinkAttrs(string(rendered))
	return out, usedLang, nil
}

// addExternalLinkAttrs post-processes HTML to add target="_blank" and rel="noopener noreferrer"
// to external links (those that start with http:// or https://), leaving internal
// anchor links (#...) and relative paths untouched.
// Uses regex to match <a> tags with external hrefs and add attributes if not present.
func addExternalLinkAttrs(htmlContent string) string {
	// Match <a href="http..."> or <a href="https..."> tags
	// This regex matches the opening tag and captures attributes to check for existing target/rel
	re := regexp.MustCompile(`(<a\s+[^>]*href=["'](https?:[^"']*)["'][^>]*>)`)

	return re.ReplaceAllStringFunc(htmlContent, func(match string) string {
		// Check if target or rel already exist
		hasTarget := regexp.MustCompile(`\starget\s*=`).MatchString(match)
		hasRel := regexp.MustCompile(`\srel\s*=`).MatchString(match)

		// Insert attributes after href attribute
		if !hasTarget && !hasRel {
			// Insert both after the href attribute
			return regexp.MustCompile(`(href=["'][^"']*["'])`).ReplaceAllString(match, `$1 target="_blank" rel="noopener noreferrer"`)
		} else if !hasTarget {
			return regexp.MustCompile(`(href=["'][^"']*["'])`).ReplaceAllString(match, `$1 target="_blank"`)
		} else if !hasRel {
			return regexp.MustCompile(`(href=["'][^"']*["'])`).ReplaceAllString(match, `$1 rel="noopener noreferrer"`)
		}
		return match
	})
}

// findFile returns the absolute path to the doc file, falling back to English.
func (h *DocsAPIHandler) findFile(docType, lang string) (string, string) {
	if h.docsDir == "" {
		return "", ""
	}
	absDir, err := filepath.Abs(h.docsDir)
	if err != nil {
		return "", ""
	}
	tryLang := func(l string) string {
		name := docType + "." + l + ".md"
		abs, err := filepath.Abs(filepath.Join(h.docsDir, name))
		if err != nil {
			return ""
		}
		if !strings.HasPrefix(abs, absDir+string(os.PathSeparator)) {
			return "" // path traversal guard
		}
		if _, err := os.Stat(abs); err == nil {
			return abs
		}
		return ""
	}
	if f := tryLang(lang); f != "" {
		return f, lang
	}
	if lang != "en" {
		if f := tryLang("en"); f != "" {
			return f, "en"
		}
	}
	return "", ""
}

// sanitizeDocLang accepts only two-letter lowercase alpha codes.
func sanitizeDocLang(lang string) string {
	lang = strings.ToLower(strings.TrimSpace(lang))
	if len(lang) == 2 && lang[0] >= 'a' && lang[0] <= 'z' && lang[1] >= 'a' && lang[1] <= 'z' {
		return lang
	}
	return ""
}

// detectLang reads the Accept-Language header and returns the first two-letter code.
func detectLang(r *http.Request) string {
	al := r.Header.Get("Accept-Language")
	if al == "" {
		return "en"
	}
	// e.g. "fr-FR,fr;q=0.9,en;q=0.8"
	first := strings.Split(al, ",")[0]
	code := strings.Split(first, "-")[0]
	if lang := sanitizeDocLang(code); lang != "" {
		return lang
	}
	return "en"
}

// findAPIDocsDir resolves the docs directory relative to the binary or source.
func findAPIDocsDir() (string, error) {
	candidates := []string{
		"/app/backend/docs",
		"./docs",
		"../docs",
		"./backend/docs",
	}
	if _, file, _, ok := runtime.Caller(0); ok {
		src := filepath.Dir(filepath.Dir(file)) // backend/
		candidates = append(candidates, filepath.Join(src, "docs"))
	}
	for _, p := range candidates {
		abs, err := filepath.Abs(p)
		if err != nil {
			continue
		}
		if _, err := os.Stat(abs); err == nil {
			return abs, nil
		}
	}
	return "", os.ErrNotExist
}

// parseClaims extracts JWT claims from the access_token cookie without going through middleware.
func (h *DocsAPIHandler) parseClaims(r *http.Request) (*JWTClaims, bool) {
	if h.state == nil {
		return nil, false
	}
	secret := h.state.GetSettings().JWTSecret
	if secret == "" {
		return nil, false
	}
	cookie, err := r.Cookie("access_token")
	if err != nil {
		return nil, false
	}
	claims := &JWTClaims{}
	token, err := jwt.ParseWithClaims(cookie.Value, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		return nil, false
	}
	return claims, true
}
