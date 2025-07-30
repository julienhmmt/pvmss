package security

import (
	"context"
	"net/http"
	"strings"

	"pvmss/logger"
)

// IsAdminRoute détermine si une route fait partie de la section admin
func IsAdminRoute(path string) bool {
	return strings.HasPrefix(path, "/admin") ||
		strings.HasPrefix(path, "/settings") ||
		strings.HasPrefix(path, "/tags") ||
		path == "/api/storage"
}

// AdminCSRFMiddleware génère des tokens CSRF pour tous mais ne les valide que pour les routes admin
func AdminCSRFMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log := logger.Get().With().
			Str("middleware", "AdminCSRF").
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Str("remote_addr", r.RemoteAddr).
			Logger()

		// Pour les méthodes sécurisées, on génère un token et on continue.
		if r.Method == "GET" || r.Method == "HEAD" || r.Method == "OPTIONS" {
			// Générer un token CSRF pour la réponse (utilisé dans les templates)
			csrfToken := GenerateCSRFToken(r)
			if csrfToken != "" {
				r = r.WithContext(context.WithValue(r.Context(), "csrf_token", csrfToken))
				log.Debug().
					Str("csrf_token", csrfToken).
					Msg("Token CSRF généré et ajouté au contexte pour la réponse")
			}

			next.ServeHTTP(w, r)
			return
		}

		// Vérifier si c'est une route admin nécessitant une validation CSRF
		if IsAdminRoute(r.URL.Path) && r.Method != "GET" {
			log.Debug().Msg("Route admin détectée, validation CSRF requise")

			// Validation du token CSRF
			isValid := ValidateCSRFToken(r)
			if !isValid {
				log.Warn().
					Str("ip", r.RemoteAddr).
					Str("path", r.URL.Path).
					Msg("Échec de la validation du token CSRF pour une route admin")
				http.Error(w, "Invalid CSRF token", http.StatusForbidden)
				return
			}

			log.Debug().Msg("Validation CSRF réussie pour route admin")
		} else {
			log.Debug().Msg("Route non-admin, CSRF non validé")
		}

		// Passer au handler suivant
		next.ServeHTTP(w, r)
	})
}
