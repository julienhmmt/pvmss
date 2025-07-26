package auth

import (
	"net/http"
	"time"
)

// LogoutHandler handles user logout
func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	// Expire the session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Path:     "/",
		SameSite: http.SameSiteStrictMode,
		Secure:   true,
	})

	// Redirect to login page
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
