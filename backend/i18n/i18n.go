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

// messageIDs holds the set of all translation keys discovered from the TOML files.
var (
	i18nDir        string // Discovered path to the i18n directory.
	supportedTags  []language.Tag
	supportedCodes map[string]struct{}
	langFileRegex  = regexp.MustCompile(`^active\.([a-z]{2}(?:-[a-z]{2})?)\.toml$`)
)

// GetLocalizer returns a localizer for the given language.
func GetLocalizer(lang string) *i18n.Localizer {
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

// GetLocalizerFromRequest creates a new localizer for the language specified in the request.
func GetLocalizerFromRequest(r *http.Request) *i18n.Localizer {
	return GetLocalizer(GetLanguage(r))
}

// Localize translates a message ID to the specified language via the provided localizer.
// If the translation fails, it returns the message ID and logs a warning.
func Localize(localizer *i18n.Localizer, messageID string, count ...int) string {
	if localizer == nil || messageID == "" {
		return messageID
	}

	if len(count) > 0 {
		// Handle pluralization if a count is provided.
		localized, err := localizer.Localize(&i18n.LocalizeConfig{MessageID: messageID, PluralCount: count[0]})
		if err != nil {
			logger.Get().Warn().Err(err).Str("message_id", messageID).Msg("Plural translation not found")
			return messageID
		}
		return localized
	}

	// Handle simple, non-plural translation.
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

	data["Lang"] = lang
	return data
}

// translationSearchPaths returns the ordered list of locations to look for translation files.
func translationSearchPaths(filename string) []string {
	return []string{
		// --- Container paths ---
		// Canonical path in the final Docker image.
		filepath.Join("/app", "backend", "i18n", filename),

		// --- Local development paths ---
		// From project root (e.g., `go run ./backend`)
		filepath.Join("backend", "i18n", filename),
		// From inside backend/ (e.g., `go run .`)
		filepath.Join("i18n", filename),
	}
}

// findI18nDirectory searches for the 'i18n' directory in common locations.
func findI18nDirectory() (string, error) {
	// Add current working directory as a search path for tests
	searchPaths := []string{
		// --- Container paths ---
		filepath.Join("/app", "backend", "i18n"),

		// --- Local development paths ---
		filepath.Join("backend", "i18n"), // From project root (e.g., `go run ./backend`)
		filepath.Join("i18n"),            // From inside backend/ (e.g., `go run .`)
		"i18n",                           // Current directory

		// --- Test environment paths ---
		filepath.Join("..", "backend", "i18n"), // From tests/ directory
		filepath.Join("..", "i18n"),            // From tests/ directory (alternative)
	}

	// Also try relative to current working directory
	if cwd, err := os.Getwd(); err == nil {
		searchPaths = append(searchPaths, []string{
			filepath.Join(cwd, "backend", "i18n"),
			filepath.Join(cwd, "i18n"),
			filepath.Join(cwd, "..", "backend", "i18n"),
			filepath.Join(cwd, "..", "i18n"),
		}...)
	}

	for _, path := range searchPaths {
		info, err := os.Stat(path)
		if (err == nil || !os.IsNotExist(err)) && info.IsDir() {
			absPath, _ := filepath.Abs(path)
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
		// Check if we're in test mode
		testMode := os.Getenv("GO_TEST_ENVIRONMENT") != "" || strings.Contains(os.Args[0], ".test")
		if testMode {
			logger.Get().Warn().Msg("i18n directory not found in test environment, continuing without translations")
			// Create a minimal bundle for tests
			Bundle = i18n.NewBundle(language.English)
			Bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)
			return
		}
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

	logger.Get().Info().Strs("languages", mapsKeys(supportedCodes)).Msg("Initialized i18n for languages")
}

func mapsKeys[M ~map[K]V, K comparable, V any](m M) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, fmt.Sprintf("%v", k))
	}
	return keys
}
