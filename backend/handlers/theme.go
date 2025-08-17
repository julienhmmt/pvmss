package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"pvmss/logger"
	"pvmss/state"
)

type ThemeHandler struct {
	state state.StateManager
}

func NewThemeHandler(sm state.StateManager) *ThemeHandler {
	return &ThemeHandler{state: sm}
}

func (h *ThemeHandler) RegisterRoutes(router *httprouter.Router) {
	router.GET("/theme/toggle", h.Toggle)
	router.GET("/theme/set/:theme", h.Set)
}

func (h *ThemeHandler) Toggle(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	current := "light"
	if c, err := r.Cookie("theme"); err == nil {
		if c.Value == "dark" || c.Value == "light" {
			current = c.Value
		}
	}
	if current == "dark" {
		setThemeCookie(w, r, "light")
	} else {
		setThemeCookie(w, r, "dark")
	}
	redirectBack(w, r)
}

func (h *ThemeHandler) Set(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	val := ps.ByName("theme")
	if val != "light" && val != "dark" {
		val = "light"
	}
	setThemeCookie(w, r, val)
	redirectBack(w, r)
}

func setThemeCookie(w http.ResponseWriter, r *http.Request, value string) {
	cookie := &http.Cookie{
		Name:     "theme",
		Value:    value,
		Path:     "/",
		Expires:  time.Now().Add(365 * 24 * time.Hour),
		MaxAge:   365 * 24 * 60 * 60,
		Secure:   r.TLS != nil,
		HttpOnly: false, // accessible to client for progressive enhancement if needed
		SameSite: http.SameSiteLaxMode,
	}
	h := logger.Get().With().Str("component", "ThemeHandler").Logger()
	h.Debug().Str("theme", value).Msg("Setting theme cookie")
	http.SetCookie(w, cookie)
}

func redirectBack(w http.ResponseWriter, r *http.Request) {
	// Explicit return path has priority
	ret := r.URL.Query().Get("return")
	if ret == "" {
		ret = r.Referer()
	}
	if ret == "" || !strings.HasPrefix(ret, "/") {
		ret = "/"
	}
	http.Redirect(w, r, ret, http.StatusSeeOther)
}
