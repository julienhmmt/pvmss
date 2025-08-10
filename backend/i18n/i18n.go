// Package i18n centralizes internationalization helpers and resources.
// It initializes the `go-i18n` bundle, determines the active language from
// requests, and provides helpers to create localizers and inject translations
// into template data.
package i18n

import (
	"bufio"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"

	"pvmss/logger"
)

// Common constants used by the i18n package.
const (
	// CookieNameLang is the cookie storing the user's preferred language.
	CookieNameLang = "pvmss_lang"
	// QueryParamLang is the URL query parameter to switch language.
	QueryParamLang = "lang"
	// HeaderAcceptLanguage is the HTTP header used for language negotiation.
	HeaderAcceptLanguage = "Accept-Language"
	// DefaultLang is the fallback language when none is specified.
	DefaultLang = "en"
	// CookieMaxAge defines the language cookie lifetime.
	CookieMaxAge = 365 * 24 * time.Hour
)

// Bundle is the global i18n bundle.
// It holds all the message catalogs for different languages and is used to create localizers.
// It is used by localizers to find and format translated strings.
var Bundle *i18n.Bundle

// translationsCache stores the fully built translation map per language to avoid
// rebuilding it on every request. Treat the cached maps as read-only.
var (
	cacheMu           sync.RWMutex
	translationsCache = make(map[string]map[string]string)
)

// messageIDs holds the set of all translation keys discovered from the TOML files.
// Populated once during InitI18n to avoid scanning files on each request.
var messageIDs []string

// supported language tags and codes
var (
	supportedTags  = []language.Tag{language.English, language.French}
	supportedCodes = map[string]struct{}{"en": {}, "fr": {}}
)

// setLanguageSwitcher populates LangEN and LangFR URLs into the provided data map.
func setLanguageSwitcher(r *http.Request, data map[string]interface{}) {
	q := r.URL.Query()
	q.Set(QueryParamLang, "en")
	data["LangEN"] = r.URL.Path + "?" + q.Encode()
	q.Set(QueryParamLang, "fr")
	data["LangFR"] = r.URL.Path + "?" + q.Encode()
}

// getLocalizer creates a new localizer for the specified language.
// If no language is provided, it falls back to the default language.
func getLocalizer(lang string) *i18n.Localizer {
	if lang == "" {
		lang = DefaultLang
	}
	// Ensure bundle is initialized
	if Bundle == nil {
		InitI18n()
	}
	// Provide fallback to default language so missing keys resolve to English
	return i18n.NewLocalizer(Bundle, lang, DefaultLang)
}

// GetLocalizer creates a new localizer for the language specified in the request.
func GetLocalizer(r *http.Request) *i18n.Localizer {
	return getLocalizer(GetLanguage(r))
}

// Localize translates a message ID to the specified language via the provided localizer.
// If the translation fails, it returns the message ID and logs a warning.
func Localize(localizer *i18n.Localizer, messageID string) string {
	if localizer == nil || messageID == "" {
		return messageID
	}

	localized, err := localizer.Localize(&i18n.LocalizeConfig{MessageID: messageID})
	if err != nil {
		logger.Get().Warn().Err(err).Str("message_id", messageID).Msg("Translation not found")
		return messageID
	}
	return localized
}

// GetLanguage extracts the language from the request: param > cookie > Accept-Language > default.
func GetLanguage(r *http.Request) string {
	// Helper to normalize and validate code
	normalize := func(code string) string {
		code = strings.TrimSpace(strings.ToLower(code))
		if len(code) > 2 {
			// reduce to base language (e.g., en-US -> en)
			if strings.Contains(code, "-") {
				code = strings.Split(code, "-")[0]
			}
		}
		if _, ok := supportedCodes[code]; ok {
			return code
		}
		return DefaultLang
	}

	// Check URL parameter first
	if lang := strings.TrimSpace(r.URL.Query().Get(QueryParamLang)); lang != "" {
		return normalize(lang)
	}

	// Check cookie
	if cookie, err := r.Cookie(CookieNameLang); err == nil && strings.TrimSpace(cookie.Value) != "" {
		return normalize(cookie.Value)
	}

	// Check Accept-Language header with robust matcher
	acceptLang := strings.TrimSpace(r.Header.Get(HeaderAcceptLanguage))
	if acceptLang != "" {
		if tags, _, err := language.ParseAcceptLanguage(acceptLang); err == nil {
			matcher := language.NewMatcher(supportedTags)
			tag, _, _ := matcher.Match(tags...)
			base, _ := tag.Base()
			code := base.String()
			if _, ok := supportedCodes[code]; ok {
				return code
			}
		}
	}

	// Default to English
	return DefaultLang
}

// GetI18nData returns i18n data for the specified language by running LocalizePage on a fake request.
func GetI18nData(lang string) map[string]interface{} {
	data := make(map[string]interface{})

	req, err := http.NewRequest(http.MethodGet, "/", nil)
	if err != nil {
		logger.Get().Warn().Err(err).Msg("failed to create request for GetI18nData")
		return data
	}
	q := req.URL.Query()
	q.Add(QueryParamLang, lang)
	req.URL.RawQuery = q.Encode()

	LocalizePage(nil, req, data)
	data["Lang"] = lang
	return data
}

// cloneMap creates a shallow copy of a string map.
func cloneMap(src map[string]string) map[string]string {
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// resolveTranslationFilePath returns the first existing path for a translation file
// using the same search strategy as loadTranslationFile.
func resolveTranslationFilePath(filename string) (string, bool) {
	possiblePaths := translationSearchPaths(filename)

	for _, p := range possiblePaths {
		if _, err := os.Stat(p); err == nil {
			return p, true
		}
	}
	return "", false
}

// translationSearchPaths returns the ordered list of locations to look for translation files.
func translationSearchPaths(filename string) []string {
	return []string{
		// Local execution from project root
		filepath.Join("backend", "i18n", filename),
		// Local execution from backend/
		filepath.Join("i18n", filename),
		// Container default path
		filepath.Join("/app", "i18n", filename),
		// Container path when repo kept under /app/backend
		filepath.Join("/app", "backend", "i18n", filename),
		// Parent dir (when starting inside backend/)
		filepath.Join("..", "i18n", filename),
	}
}

// extractMessageIDsFromFile scans a TOML translation file and extracts message IDs
// defined as headings like ["Section.Key"].
func extractMessageIDsFromFile(path string) []string {
	f, err := os.Open(path)
	if err != nil {
		logger.Get().Warn().Err(err).Str("file", path).Msg("unable to open translation file to extract keys")
		return nil
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	ids := make([]string, 0, 512)
	seen := make(map[string]struct{})
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(line) == 0 || line[0] != '[' {
			continue
		}
		// expect ["..."]
		if strings.HasPrefix(line, "[\"") && strings.Contains(line, "\"]") {
			start := len("[\"")
			end := strings.Index(line[start:], "\"]")
			if end > 0 {
				id := line[start : start+end]
				if id != "" {
					if _, ok := seen[id]; !ok {
						seen[id] = struct{}{}
						ids = append(ids, id)
					}
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		logger.Get().Warn().Err(err).Str("file", path).Msg("scanner error while extracting keys")
	}
	return ids
}

// getAllMessageIDs tries to read message IDs from available locale files.
func getAllMessageIDs() []string {
	combined := make([]string, 0, 1024)
	seen := make(map[string]struct{})
	// Prefer English file as baseline
	if p, ok := resolveTranslationFilePath("active.en.toml"); ok {
		for _, id := range extractMessageIDsFromFile(p) {
			if _, ex := seen[id]; !ex {
				seen[id] = struct{}{}
				combined = append(combined, id)
			}
		}
	}
	// Merge any additional ids from FR
	if p, ok := resolveTranslationFilePath("active.fr.toml"); ok {
		for _, id := range extractMessageIDsFromFile(p) {
			if _, ex := seen[id]; !ex {
				seen[id] = struct{}{}
				combined = append(combined, id)
			}
		}
	}
	return combined
}

// loadTranslationFile attempts to load a translation file from several possible
// locations to accommodate different execution environments (local, container, parent dir).
func loadTranslationFile(bundle *i18n.Bundle, filename string) {
	possiblePaths := translationSearchPaths(filename)

	var loaded bool
	var lastError error

	for _, path := range possiblePaths {
		absPath, _ := filepath.Abs(path)
		logger.Get().Debug().Str("path", absPath).Msgf("Attempting to load translation file: %s", filename)

		if _, err := os.Stat(path); err == nil {
			if _, err := bundle.LoadMessageFile(path); err == nil {
				logger.Get().Info().Str("file", absPath).Msg("Translation file loaded successfully")
				loaded = true
				break
			} else {
				lastError = err
				logger.Get().Warn().Err(err).Str("path", absPath).Msg("Error while loading translation file")
			}
		} else {
			logger.Get().Debug().Str("path", absPath).Msg("Translation file not found at this location")
		}
	}

	if !loaded {
		logger.Get().Error().
			Str("file", filename).
			Err(lastError).
			Msg("Failed to load translation file from any known location")
	}
}

// InitI18n initializes the internationalization bundle.
// It sets English as the default language, registers the TOML unmarshal function,
// and loads translation files for supported locales.
func InitI18n() {
	// Create a new bundle with English as default language
	Bundle = i18n.NewBundle(language.English)
	Bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)

	// Load supported locales
	loadTranslationFile(Bundle, "active.en.toml")
	loadTranslationFile(Bundle, "active.fr.toml")

	// Discover message IDs once
	messageIDs = getAllMessageIDs()

	// Pre-warm cache for supported languages
	for code := range supportedCodes {
		loc := i18n.NewLocalizer(Bundle, code, DefaultLang)
		t := make(map[string]string, len(messageIDs))
		for _, id := range messageIDs {
			t[id] = Localize(loc, id)
		}
		cacheMu.Lock()
		translationsCache[code] = cloneMap(t)
		cacheMu.Unlock()
	}

	// Diagnostics: log a few sample keys to confirm catalogs
	enLoc := i18n.NewLocalizer(Bundle, "en")
	frLoc := i18n.NewLocalizer(Bundle, "fr", DefaultLang)
	samples := []string{"Navbar.Home", "Common.Login", "UI.Button.Search"}
	for _, k := range samples {
		enVal := Localize(enLoc, k)
		frVal := Localize(frLoc, k)
		logger.Get().Debug().Str("key", k).Str("en", enVal).Str("fr", frVal).Msg("i18n sample")
	}
}

// LocalizePage injects all necessary localized strings into the data map for a given request.
// This function is called by renderTemplate before executing a template to ensure all UI text is translated.
// It uses the Localize function to safely handle missing translations.
func LocalizePage(w http.ResponseWriter, r *http.Request, data map[string]interface{}) {
	if data == nil {
		data = make(map[string]interface{})
	}

	// Get language from request and set cookie if needed
	lang := GetLanguage(r)
	if langParam := strings.TrimSpace(r.URL.Query().Get(QueryParamLang)); langParam != "" {
		if w != nil {
			http.SetCookie(w, &http.Cookie{
				Name:     CookieNameLang,
				Value:    langParam,
				Path:     "/",
				Expires:  time.Now().Add(CookieMaxAge),
				SameSite: http.SameSiteLaxMode,
			})
		}
		lang = langParam
	}

	// Expose current language and log resolution source for diagnostics
	data["Lang"] = lang
	source := "default"
	if strings.TrimSpace(r.URL.Query().Get(QueryParamLang)) != "" {
		source = "param"
	} else if cookie, err := r.Cookie(CookieNameLang); err == nil && strings.TrimSpace(cookie.Value) != "" {
		source = "cookie"
	} else if strings.TrimSpace(r.Header.Get(HeaderAcceptLanguage)) != "" {
		source = "header"
	}
	logger.Get().Debug().Str("lang", lang).Str("source", source).Msg("Resolved language")

	// Set Content-Language header for clients/proxies
	if w != nil {
		w.Header().Set("Content-Language", lang)
	}

	// Serve from cache if available
	cacheMu.RLock()
	if cached, ok := translationsCache[lang]; ok {
		cacheMu.RUnlock()
		data["t"] = cloneMap(cached)
		setLanguageSwitcher(r, data)
		return
	}
	cacheMu.RUnlock()

	// Create localizer for the current language
	localizer := getLocalizer(lang)

	// Safety net: if messageIDs were not discovered at init (e.g., unusual runtime path), recompute once
	if len(messageIDs) == 0 {
		messageIDs = getAllMessageIDs()
	}
	t := make(map[string]string, len(messageIDs))

	// Populate translation keys discovered at init
	for _, id := range messageIDs {
		t[id] = Localize(localizer, id)
	}

	// Add the completed translation map to the main data map
	data["t"] = t

	// Store in cache for this language
	cacheMu.Lock()
	translationsCache[lang] = cloneMap(t)
	cacheMu.Unlock()

	setLanguageSwitcher(r, data)
}
