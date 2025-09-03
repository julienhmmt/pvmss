// Package i18n centralizes internationalization helpers and resources.
// It initializes the `go-i18n` bundle, determines the active language from
// requests, and provides helpers to create localizers and inject translations
// into template data.
package i18n

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"

	"pvmss/logger"
)

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

var (
	i18nDir        string // Discovered path to the i18n directory.
	supportedTags  []language.Tag
	supportedCodes map[string]struct{}
	langFileRegex  = regexp.MustCompile(`^active\.([a-z]{2}(?:-[a-z]{2})?)\.toml$`)
)

// setLanguageSwitcher populates language switcher URLs into the provided data map.
func setLanguageSwitcher(r *http.Request, data map[string]interface{}) {
	q := r.URL.Query()
	for code := range supportedCodes {
		q.Set(QueryParamLang, code)
		key := fmt.Sprintf("Lang%s", strings.ToUpper(code))
		data[key] = r.URL.Path + "?" + q.Encode()
	}
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

// extractMessageIDsFromMap recursively traverses a map to build a flat list of fully-qualified message IDs.
func extractMessageIDsFromMap(m map[string]interface{}, prefix string, ids *[]string) {
	for k, v := range m {
		newPrefix := k
		if prefix != "" {
			newPrefix = prefix + "." + k
		}

		if subMap, ok := v.(map[string]interface{}); ok {
			extractMessageIDsFromMap(subMap, newPrefix, ids)
		} else {
			*ids = append(*ids, newPrefix)
		}
	}
}

// parseMessageIDsFromFile opens and parses a TOML file, then extracts all message IDs.
func parseMessageIDsFromFile(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		logger.Get().Warn().Err(err).Str("file", path).Msg("unable to read translation file to extract keys")
		return nil
	}

	var m map[string]interface{}
	if err := toml.Unmarshal(data, &m); err != nil {
		logger.Get().Warn().Err(err).Str("file", path).Msg("failed to parse TOML for key extraction")
		return nil
	}

	var ids []string
	extractMessageIDsFromMap(m, "", &ids)
	return ids
}

// getAllMessageIDs discovers all unique message IDs from all available translation files.
func getAllMessageIDs() []string {
	combined := make(map[string]struct{})
	files, err := filepath.Glob(filepath.Join(i18nDir, "active.*.toml"))
	if err != nil {
		logger.Get().Error().Err(err).Msg("Failed to glob for translation files")
		return nil
	}

	for _, file := range files {
		for _, id := range parseMessageIDsFromFile(file) {
			combined[id] = struct{}{}
		}
	}

	uniqueIDs := make([]string, 0, len(combined))
	for id := range combined {
		uniqueIDs = append(uniqueIDs, id)
	}
	return uniqueIDs
}

// findI18nDirectory searches for the 'i18n' directory in common locations.
func findI18nDirectory() (string, error) {
	for _, path := range translationSearchPaths("") {
		// We are looking for the directory itself, so use filepath.Dir
		dir := filepath.Dir(path)
		if _, err := os.Stat(dir); err == nil {
			absPath, _ := filepath.Abs(dir)
			logger.Get().Info().Str("path", absPath).Msg("Found i18n directory")
			return absPath, nil
		}
	}
	return "", fmt.Errorf("i18n directory not found in any search paths")
}

// loadAllTranslations discovers and loads all 'active.*.toml' files.
func loadAllTranslations(bundle *i18n.Bundle) {
	files, err := filepath.Glob(filepath.Join(i18nDir, "active.*.toml"))
	if err != nil {
		logger.Get().Error().Err(err).Msg("Failed to glob for translation files")
		return
	}

	if len(files) == 0 {
		logger.Get().Error().Msg("No translation files found.")
		return
	}

	for _, file := range files {
		if _, err := bundle.LoadMessageFile(file); err != nil {
			logger.Get().Error().Err(err).Str("file", file).Msg("Failed to load translation file")
		}
		logger.Get().Info().Str("file", file).Msg("Translation file loaded successfully")
	}
}

// InitI18n discovers languages, loads translations, and pre-warms the cache.
func InitI18n() {
	var err error
	i18nDir, err = findI18nDirectory()
	if err != nil {
		logger.Get().Fatal().Err(err).Msg("Could not initialize i18n")
	}

	// Dynamically discover supported languages
	supportedCodes = make(map[string]struct{})
	files, _ := filepath.Glob(filepath.Join(i18nDir, "active.*.toml"))
	for _, file := range files {
		matches := langFileRegex.FindStringSubmatch(filepath.Base(file))
		if len(matches) > 1 {
			code := matches[1]
			tag, err := language.Parse(code)
			if err != nil {
				logger.Get().Warn().Str("code", code).Msg("Skipping invalid language code")
				continue
			}
			supportedTags = append(supportedTags, tag)
			supportedCodes[code] = struct{}{}
		}
	}

	// Create a new bundle with English as default language
	Bundle = i18n.NewBundle(language.English)
	Bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)

	// Load all discovered translation files
	loadAllTranslations(Bundle)

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

	// Log diagnostics
	logger.Get().Info().Int("count", len(messageIDs)).Msg("Discovered message IDs")
	logger.Get().Info().Strs("languages", mapsKeys(supportedCodes)).Msg("Initialized i18n for languages")
}

func mapsKeys[M ~map[K]V, K comparable, V any](m M) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, fmt.Sprintf("%v", k))
	}
	return keys
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
