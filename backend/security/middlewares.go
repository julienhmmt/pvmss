package security

import (
	"net/http"
)

// CSRFMiddleware est remplacé par AdminCSRFMiddleware
// pour cibler uniquement la partie admin et éviter l'interférence avec Proxmox.
// Voir admin_csrf.go pour la nouvelle implémentation.

// HeadersMiddleware ajoute des en-têtes de sécurité à toutes les réponses
func HeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// En-têtes de sécurité de base
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		next.ServeHTTP(w, r)
	})
}

// HeadersMiddleware adds security headers
// func HeadersMiddleware(next http.Handler) http.Handler {
// 	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		// Security headers
// 		w.Header().Set("X-Content-Type-Options", "nosniff")
// 		w.Header().Set("X-Frame-Options", "DENY")
// 		w.Header().Set("X-XSS-Protection", "1; mode=block")
// 		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

// 		// Content Security Policy
// 		csp := "default-src 'self'; " +
// 			"script-src 'self' 'unsafe-inline'; " +
// 			"style-src 'self' 'unsafe-inline'; " +
// 			"img-src 'self' data:; " +
// 			"font-src 'self'; " +
// 			"connect-src 'self'"
// 		w.Header().Set("Content-Security-Policy", csp)

// 		next.ServeHTTP(w, r)
// 	})
// }
