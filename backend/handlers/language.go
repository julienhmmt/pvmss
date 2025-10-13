package handlers

import (
	"net/http"
	"net/url"
	"time"

	"github.com/julienschmidt/httprouter"

	"pvmss/i18n"
)

// LanguageHandler handles language switching via cookies
type LanguageHandler struct{}

// NewLanguageHandler creates a new LanguageHandler instance
func NewLanguageHandler() *LanguageHandler {
	return &LanguageHandler{}
}

// SetLanguage handles language switching requests and redirects back
func (h *LanguageHandler) SetLanguage(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("LanguageHandler.SetLanguage", r)

	// Get the requested language from query parameter
	lang := r.URL.Query().Get("lang")
	if lang == "" {
		lang = "en"
	}

	// Validate and normalize the language code
	lang = i18n.GetLanguage(&http.Request{
		URL: r.URL,
	})

	log.Debug().Str("language", lang).Msg("Setting language cookie")

	// Set the language cookie
	http.SetCookie(w, &http.Cookie{
		Name:     i18n.CookieNameLang,
		Value:    lang,
		Path:     "/",
		MaxAge:   int(i18n.CookieMaxAge / time.Second),
		HttpOnly: false, // Allow JavaScript to read for client-side functionality
		SameSite: http.SameSiteLaxMode,
	})

	// Get the return URL from query parameter or referer
	returnURL := r.URL.Query().Get("return")
	if returnURL == "" {
		referer := r.Header.Get("Referer")
		if referer != "" {
			// Parse referer to extract just the path
			if parsed, err := url.Parse(referer); err == nil && parsed.Path != "" {
				returnURL = parsed.Path
				if parsed.RawQuery != "" {
					returnURL += "?" + parsed.RawQuery
				}
			} else {
				returnURL = "/"
			}
		} else {
			returnURL = "/"
		}
	} else {
		// Ensure return URL is a local path
		returnURL = ensureLocalPath(returnURL)
	}

	log.Debug().Str("return_url", returnURL).Msg("Redirecting after language change")

	// Redirect back to the referring page
	http.Redirect(w, r, returnURL, http.StatusSeeOther)
}

// RegisterRoutes registers language-related routes
func (h *LanguageHandler) RegisterRoutes(router *httprouter.Router) {
	router.GET("/set-lang", h.SetLanguage)
}
