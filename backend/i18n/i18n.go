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
	possiblePaths := []string{
		filepath.Join("i18n", filename),
		filepath.Join("/app", "i18n", filename),
		filepath.Join("..", "i18n", filename),
	}

	for _, p := range possiblePaths {
		if _, err := os.Stat(p); err == nil {
			return p, true
		}
	}
	return "", false
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
	possiblePaths := []string{
		// Emplacement standard (pour l'exécution locale)
		filepath.Join("i18n", filename),
		// Emplacement dans le conteneur
		filepath.Join("/app", "i18n", filename),
		// Emplacement parent (si exécuté depuis le dossier backend)
		filepath.Join("..", "i18n", filename),
	}

	var loaded bool
	var lastError error

	for _, path := range possiblePaths {
		absPath, _ := filepath.Abs(path)
		logger.Get().Debug().Str("path", absPath).Msgf("Tentative de chargement du fichier de traduction: %s", filename)

		if _, err := os.Stat(path); err == nil {
			if _, err := bundle.LoadMessageFile(path); err == nil {
				logger.Get().Info().Str("file", absPath).Msg("Fichier de traduction chargé avec succès")
				loaded = true
				break
			} else {
				lastError = err
				logger.Get().Warn().Err(err).Str("path", absPath).Msg("Erreur lors du chargement du fichier de traduction")
			}
		} else {
			logger.Get().Debug().Str("path", absPath).Msg("Fichier de traduction introuvable à cet emplacement")
		}
	}

	if !loaded {
		logger.Get().Error().
			Str("file", filename).
			Err(lastError).
			Msg("Impossible de charger le fichier de traduction depuis aucun des emplacements connus")
	}
}

// InitI18n initializes the internationalization bundle.
// It sets English as the default language, registers the TOML unmarshal function,
// and loads translation files for supported locales.
func InitI18n() {
	// Créer un nouveau bundle avec l'anglais comme langue par défaut
	Bundle = i18n.NewBundle(language.English)
	Bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)

	// Charger les traductions anglaises
	loadTranslationFile(Bundle, "active.en.toml")

	// Charger les traductions françaises
	loadTranslationFile(Bundle, "active.fr.toml")
}

// getLocalizer creates a new localizer for the specified language.
// If no language is provided, it falls back to the default language.
func getLocalizer(lang string) *i18n.Localizer {
	if lang == "" {
		lang = "en" // Default to English
	}
	return i18n.NewLocalizer(Bundle, lang)
}

// GetLocalizer creates a new localizer for the language specified in the request.
func GetLocalizer(r *http.Request) *i18n.Localizer {
	return getLocalizer(GetLanguage(r))
}

// Localize translates a message ID to the specified language.
// If the translation fails, it returns the message ID and logs a warning.
func Localize(localizer *i18n.Localizer, messageID string) string {
	if localizer == nil || messageID == "" {
		return messageID
	}

	localized, err := localizer.Localize(&i18n.LocalizeConfig{
		MessageID: messageID,
	})

	if err != nil {
		logger.Get().Warn().
			Err(err).
			Str("message_id", messageID).
			Msg("Translation not found")
		return messageID
	}

	return localized
}

// GetLanguage extracts the language from the request (cookie, header, or default)
func GetLanguage(r *http.Request) string {
	// Check URL parameter first
	if lang := r.URL.Query().Get(QueryParamLang); lang != "" {
		return lang
	}

	// Check cookie
	if cookie, err := r.Cookie(CookieNameLang); err == nil && cookie.Value != "" {
		return cookie.Value
	}

	// Check Accept-Language header
	acceptLang := r.Header.Get(HeaderAcceptLanguage)
	if acceptLang != "" {
		// Parse first language from header (e.g., "en-US,en;q=0.9" -> "en")
		if strings.Contains(acceptLang, ";") {
			acceptLang = strings.Split(acceptLang, ";")[0]
		}
		if strings.Contains(acceptLang, ",") {
			acceptLang = strings.Split(acceptLang, ",")[0]
		}
		if strings.Contains(acceptLang, "-") {
			acceptLang = strings.Split(acceptLang, "-")[0]
		}
		return strings.ToLower(acceptLang)
	}

	// Default to English
	return DefaultLang
}

// GetI18nData returns i18n data for the specified language
func GetI18nData(lang string) map[string]interface{} {
	data := make(map[string]interface{})

	// Create a request with the specified language
	req, err := http.NewRequest(http.MethodGet, "/", nil)
	if err != nil {
		logger.Get().Warn().Err(err).Msg("failed to create request for GetI18nData")
		return data
	}
	q := req.URL.Query()
	q.Add(QueryParamLang, lang)
	req.URL.RawQuery = q.Encode()

	// Use localizePage to populate translations
	LocalizePage(nil, req, data)
	data["Lang"] = lang
	return data
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
	if langParam := r.URL.Query().Get(QueryParamLang); langParam != "" {
		if w != nil {
			http.SetCookie(w, &http.Cookie{
				Name:    CookieNameLang,
				Value:   langParam,
				Path:    "/",
				Expires: time.Now().Add(CookieMaxAge),
			})
		}
		lang = langParam
	}

	// Serve from cache if available
	cacheMu.RLock()
	if cached, ok := translationsCache[lang]; ok {
		cacheMu.RUnlock()
		data["t"] = cloneMap(cached)
		// Add language switcher
		q := r.URL.Query()
		q.Set(QueryParamLang, "en")
		data["LangEN"] = r.URL.Path + "?" + q.Encode()
		q.Set(QueryParamLang, "fr")
		data["LangFR"] = r.URL.Path + "?" + q.Encode()
		return
	}
	cacheMu.RUnlock()

	// Create localizer for the current language
	localizer := getLocalizer(lang)

	// Create the translation map that templates expect
	t := make(map[string]string)

	// Dynamically populate translation keys from TOML files
	for _, id := range getAllMessageIDs() {
		t[id] = Localize(localizer, id)
	}

	// Add the completed translation map to the main data map
	data["t"] = t

	// Store in cache for this language
	cacheMu.Lock()
	translationsCache[lang] = cloneMap(t)
	cacheMu.Unlock()

	// Add language switcher URLs
	q := r.URL.Query()
	q.Set(QueryParamLang, "en")
	data["LangEN"] = r.URL.Path + "?" + q.Encode()
	q.Set(QueryParamLang, "fr")
	data["LangFR"] = r.URL.Path + "?" + q.Encode()
}
