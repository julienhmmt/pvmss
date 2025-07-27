package handlers

import (
	"bytes"
	"context"
	"html/template"
	"net/http"

	"pvmss/i18n"
	"pvmss/logger"
	"pvmss/state"

	"github.com/julienschmidt/httprouter"
)

// contextKey is used for context keys to avoid collisions between packages using context
type contextKey string

// ParamsKey is the key used to store httprouter.Params in the request context
const ParamsKey contextKey = "params"

// InitState est une fonction de compatibilité qui ne fait plus rien
// car nous utilisons maintenant le package state
func InitState() {
	logger.Get().Info().Msg("Handlers using global state manager")
}

// RenderTemplate affiche un template avec les données fournies
// Cette fonction est exportée pour être utilisée par d'autres packages
func RenderTemplate(w http.ResponseWriter, r *http.Request, name string, data interface{}) {
	log := logger.Get().With().
		Str("handler", "RenderTemplate").
		Str("template", name).
		Str("path", r.URL.Path).
		Str("method", r.Method).
		Logger()

	log.Debug().Msg("Début du rendu du template")

	// Convertir data en map si nécessaire
	dataMap := make(map[string]interface{})
	if data != nil {
		if dm, ok := data.(map[string]interface{}); ok {
			dataMap = dm
			log.Debug().Int("data_map_size", len(dm)).Msg("Données fournies sous forme de map")
		} else {
			dataMap["Data"] = data
			log.Debug().Type("data_type", data).Msg("Données fournies sous forme de structure, conversion en map")
		}
	} else {
		log.Debug().Msg("Aucune donnée fournie pour le rendu du template")
	}

	// Utiliser la fonction interne avec la map
	renderTemplateInternal(w, r, name, dataMap)

	log.Info().
		Str("template", name).
		Msg("Rendu du template terminé avec succès")
}

// renderTemplate est la fonction interne pour le rendu des templates
func renderTemplateInternal(w http.ResponseWriter, r *http.Request, name string, data map[string]interface{}) {
	log := logger.Get().With().
		Str("handler", "renderTemplateInternal").
		Str("template", name).
		Str("path", r.URL.Path).
		Logger()

	log.Debug().Msg("Début du rendu interne du template")

	stateManager := state.GetGlobalState()
	tmpl := stateManager.GetTemplates()

	if tmpl == nil {
		errMsg := "Les templates ne sont pas initialisés"
		log.Error().Msg(errMsg)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	// Initialiser les données si nécessaire
	if data == nil {
		log.Debug().Msg("Initialisation d'une nouvelle map de données vide")
		data = make(map[string]interface{})
	} else {
		log.Debug().Int("data_size", len(data)).Msg("Données fournies pour le rendu")
	}

	// Ajouter les données d'authentification
	if IsAuthenticated(r) {
		log.Debug().Msg("Utilisateur authentifié détecté, ajout des données de session")
		data["IsAuthenticated"] = true
		sessionManager := stateManager.GetSessionManager()
		username := sessionManager.GetString(r.Context(), "username")
		if username != "" {
			data["Username"] = username
			log.Debug().Str("username", username).Msg("Nom d'utilisateur ajouté aux données du template")
		}
	} else {
		log.Debug().Msg("Aucun utilisateur authentifié détecté")
	}

	// Ajouter les données i18n et variables communes
	log.Debug().Msg("Application des données i18n et des variables communes")
	i18n.LocalizePage(w, r, data)
	data["CurrentPath"] = r.URL.Path
	data["IsHTTPS"] = r.TLS != nil
	data["Host"] = r.Host

	log.Debug().
		Str("current_path", r.URL.Path).
		Bool("is_https", r.TLS != nil).
		Str("host", r.Host).
		Msg("Variables de contexte ajoutées")

	// Exécuter le template
	buf := new(bytes.Buffer)
	log.Debug().Msg("Exécution du template principal")

	if err := tmpl.ExecuteTemplate(buf, name, data); err != nil {
		log.Error().
			Err(err).
			Str("template", name).
			Msg("Échec de l'exécution du template")
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	log.Debug().
		Int("content_length", buf.Len()).
		Msg("Template principal exécuté avec succès")

	// Ajouter le contenu au layout
	content := buf.String()
	data["Content"] = template.HTML(content)

	log.Debug().
		Int("content_length", len(content)).
		Msg("Contenu du template préparé pour le layout")

	// Exécuter le layout
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	log.Debug().Msg("Exécution du template de layout")

	if err := tmpl.ExecuteTemplate(w, "layout", data); err != nil {
		log.Error().
			Err(err).
			Str("template", "layout").
			Msg("Échec de l'exécution du template de layout")
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	log.Info().
		Str("template", name).
		Int("response_size", len(content)).
		Msg("Rendu de la page terminé avec succès")
}

// IsAuthenticated vérifie si l'utilisateur est authentifié
// Cette fonction est exportée pour être utilisée par d'autres packages
func IsAuthenticated(r *http.Request) bool {
	log := logger.Get().With().
		Str("function", "IsAuthenticated").
		Str("remote_addr", r.RemoteAddr).
		Logger()

	stateManager := state.GetGlobalState()
	sessionManager := stateManager.GetSessionManager()

	// Vérifier si la session contient le flag d'authentification
	authenticated, ok := sessionManager.Get(r.Context(), "authenticated").(bool)
	if !ok {
		log.Debug().Msg("Aucune session d'authentification active")
		return false
	}

	log.Debug().
		Bool("is_authenticated", authenticated).
		Msg("Vérification de l'authentification")

	return authenticated
}

// RequireAuth est un middleware pour protéger les routes nécessitant une authentification
// Cette fonction est exportée pour être utilisée par d'autres packages
func RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !IsAuthenticated(r) {
			stateManager := state.GetGlobalState()
			sessionManager := stateManager.GetSessionManager()
			sessionManager.Put(r.Context(), "redirect_after_login", r.URL.Path)

			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		// Ajouter des en-têtes de sécurité
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")

		next.ServeHTTP(w, r)
	}
}

// IndexHandler est un handler pour la page d'accueil
// Cette fonction est exportée pour être utilisée par d'autres packages
func IndexHandler(w http.ResponseWriter, r *http.Request) {
	log := logger.Get().With().
		Str("handler", "IndexHandler").
		Str("path", r.URL.Path).
		Str("remote_addr", r.RemoteAddr).
		Logger()

	log.Debug().Msg("Traitement de la requête pour la page d'accueil")

	// Si ce n'est pas la racine, on renvoie une 404
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// Préparer les données pour le template
	data := map[string]interface{}{
		"Title": "PVMSS",
		"Lang":  i18n.GetLanguage(r), // Ajouter la langue détectée
	}

	// Ajouter les données de traduction en fonction de la langue
	i18n.LocalizePage(w, r, data)

	log.Debug().Msg("Rendu du template index")
	renderTemplateInternal(w, r, "index", data) // Utiliser le nom du template au lieu du nom de fichier

	log.Info().Msg("Page d'accueil affichée avec succès")
}

// IndexRouterHandler est un handler pour la page d'accueil compatible avec httprouter
func IndexRouterHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	logger.Get().Debug().
		Str("handler", "IndexRouterHandler").
		Str("path", r.URL.Path).
		Msg("Appel du gestionnaire d'index via routeur HTTP")

	// Délègue le traitement au gestionnaire principal
	IndexHandler(w, r)
}

// HandlerFuncToHTTPrHandle adapte un http.HandlerFunc à une fonction httprouter.Handle.
// Cette fonction permet d'utiliser des handlers standards avec le routeur httprouter.
func HandlerFuncToHTTPrHandle(h http.HandlerFunc) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		// Créer un logger pour cette requête
		log := logger.Get().With().
			Str("adapter", "HandlerFuncToHTTPrHandle").
			Str("path", r.URL.Path).
			Str("method", r.Method).
			Int("params_count", len(ps)).
			Logger()

		log.Debug().Msg("Adaptation du gestionnaire HTTP standard pour httprouter")

		// Ajouter les paramètres de route au contexte de la requête
		ctx := context.WithValue(r.Context(), ParamsKey, ps)

		// Appeler le gestionnaire d'origine avec le nouveau contexte
		h(w, r.WithContext(ctx))

		log.Debug().Msg("Traitement du gestionnaire HTTP terminé")
	}
}
