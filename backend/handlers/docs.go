package handlers

import (
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"pvmss/i18n"
	"pvmss/logger"

	"github.com/gomarkdown/markdown"
	"github.com/julienschmidt/httprouter"
)

// DocsHandler gère les routes de documentation
type DocsHandler struct {
	docsDir string
}

// NewDocsHandler crée une nouvelle instance de DocsHandler
func NewDocsHandler() *DocsHandler {
	log := logger.Get().With().
		Str("component", "DocsHandler").
		Str("function", "NewDocsHandler").
		Logger()

	log.Debug().Msg("Recherche du répertoire de documentation")

	docsDir, err := findDocsDir()
	if err != nil {
		log.Error().
			Err(err).
			Msg("Échec de la recherche du répertoire de documentation")
	} else {
		log.Info().
			Str("docs_dir", docsDir).
			Msg("Répertoire de documentation trouvé avec succès")
	}

	return &DocsHandler{
		docsDir: docsDir,
	}
}

// DocsHandler gère les requêtes vers la documentation
func (h *DocsHandler) DocsHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// Créer un logger pour cette requête
	log := logger.Get().With().
		Str("handler", "DocsHandler").
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("remote_addr", r.RemoteAddr).
		Logger()

	log.Debug().Msg("Traitement de la requête de documentation")

	// Vérifier si le répertoire de documentation est disponible
	if h.docsDir == "" {
		errMsg := "Aucun répertoire de documentation n'est configuré"
		log.Error().Msg(errMsg)
		http.Error(w, "Documentation non disponible", http.StatusServiceUnavailable)
		return
	}

	// Déterminer la langue (par défaut: en)
	lang := r.URL.Query().Get("lang")
	if lang == "" {
		lang = "en"
		log.Debug().Msg("Aucune langue spécifiée, utilisation de l'anglais par défaut")
	} else {
		log.Debug().Str("lang", lang).Msg("Langue spécifiée dans la requête")
	}

	// Déterminer le type de documentation (admin ou user)
	docType := ps.ByName("type")
	if docType == "" {
		docType = "user"
		log.Debug().Msg("Aucun type de documentation spécifié, utilisation du type 'user' par défaut")
	} else {
		log.Debug().Str("doc_type", docType).Msg("Type de documentation spécifié")
	}

	// Construire le chemin du fichier de documentation
	docFile := filepath.Join(h.docsDir, fmt.Sprintf("%s.%s.md", docType, lang))

	log.Debug().
		Str("requested_file", docFile).
		Msg("Recherche du fichier de documentation")

	// Vérifier si le fichier existe
	if _, err := os.Stat(docFile); os.IsNotExist(err) {
		log.Debug().
			Str("file", docFile).
			Msg("Fichier de documentation non trouvé, tentative avec la langue anglaise")

		// Essayer avec la langue par défaut si le fichier n'existe pas
		if lang != "en" {
			docFile = filepath.Join(h.docsDir, fmt.Sprintf("%s.en.md", docType))
			log.Debug().
				Str("fallback_file", docFile).
				Msg("Tentative avec le fichier de secours en anglais")

			// Vérifier à nouveau après le changement de langue
			if _, err := os.Stat(docFile); os.IsNotExist(err) {
				log.Warn().
					Str("file", docFile).
					Str("doc_type", docType).
					Str("lang", "en").
					Msg("Fichier de documentation introuvable même en anglais")

				http.NotFound(w, r)
				return
			}
		} else {
			log.Warn().
				Str("file", docFile).
				Msg("Fichier de documentation introuvable et aucune alternative disponible")

			http.NotFound(w, r)
			return
		}
	}

	// Lire le contenu du fichier Markdown
	log.Debug().
		Str("file", docFile).
		Msg("Lecture du fichier de documentation")

	content, err := os.ReadFile(docFile)
	if err != nil {
		log.Error().
			Err(err).
			Str("file", docFile).
			Msg("Échec de la lecture du fichier de documentation")

		http.Error(w, "Erreur interne du serveur lors de la lecture de la documentation", http.StatusInternalServerError)
		return
	}

	log.Debug().
		Int("file_size_bytes", len(content)).
		Msg("Contenu du fichier de documentation lu avec succès")

	// Convertir le Markdown en HTML
	htmlContent := markdown.ToHTML(content, nil, nil)

	log.Debug().
		Int("html_size_bytes", len(htmlContent)).
		Msg("Conversion Markdown vers HTML effectuée avec succès")

	// Préparer les données pour le template
	data := map[string]interface{}{
		"Content":     template.HTML(htmlContent),
		"CurrentLang": lang,
		"DocType":     docType,
	}

	log.Debug().
		Str("doc_type", docType).
		Str("lang", lang).
		Msg("Préparation des données pour le rendu du template")

	// Charger les traductions
	i18n.LocalizePage(w, r, data)
	data["Title"] = data["Docs.Title"]

	log.Debug().Msg("Appel du rendu du template de documentation")
	renderTemplateInternal(w, r, "docs", data)

	log.Info().
		Str("doc_type", docType).
		Str("lang", lang).
		Msg("Documentation affichée avec succès")
}

// findDocsDir cherche le répertoire de documentation
func findDocsDir() (string, error) {
	log := logger.Get().With().
		Str("function", "findDocsDir").
		Logger()

	log.Debug().Msg("Recherche du répertoire de documentation")

	// Obtenir le chemin du fichier source appelant
	_, filename, _, ok := runtime.Caller(0)
	if ok {
		log.Debug().
			Str("caller_file", filename).
			Msg("Fichier appelant identifié")
	}

	// Liste des emplacements possibles pour le répertoire de documentation
	possibleDirs := []string{
		"./docs",
		"../docs",
		"./backend/docs",
		"/app/backend/docs",
		"/app/docs",
	}

	// Ajouter le répertoire du binaire en cours d'exécution
	execDir, err := os.Executable()
	if err == nil {
		execDir = filepath.Dir(execDir)
		docsPath := filepath.Join(execDir, "docs")
		possibleDirs = append(possibleDirs, docsPath)
		log.Debug().
			Str("exec_dir", execDir).
			Str("docs_path", docsPath).
			Msg("Répertoire d'exécution ajouté aux emplacements de recherche")
	} else {
		log.Warn().
			Err(err).
			Msg("Impossible de déterminer le répertoire d'exécution")
	}

	// Ajouter le répertoire du fichier source
	_, filename, _, ok = runtime.Caller(0)
	if ok {
		srcDir := filepath.Dir(filepath.Dir(filename))
		possibleDirs = append(possibleDirs, filepath.Join(srcDir, "docs"))
	}

	// Vérifier chaque emplacement possible
	for _, dir := range possibleDirs {
		log.Debug().
			Str("checking_dir", dir).
			Msg("Vérification de l'emplacement de documentation")

		info, err := os.Stat(dir)
		if err == nil && info.IsDir() {
			log.Info().
				Str("found_dir", dir).
				Msg("Répertoire de documentation trouvé")

			// Vérifier que le répertoire contient des fichiers de documentation
			entries, err := os.ReadDir(dir)
			if err == nil && len(entries) > 0 {
				log.Info().
					Str("dir", dir).
					Int("file_count", len(entries)).
					Msg("Répertoire de documentation valide avec fichiers trouvés")
				return dir, nil
			}

			log.Warn().
				Str("dir", dir).
				Msg("Répertoire trouvé mais vide ou inaccessible")
		}
	}

	// Si aucun répertoire valide n'a été trouvé
	return "", fmt.Errorf("impossible de trouver le répertoire de documentation dans les emplacements suivants: %v", possibleDirs)
}

// RegisterRoutes enregistre les routes de documentation
func (h *DocsHandler) RegisterRoutes(router *httprouter.Router) {
	log := logger.Get().With().
		Str("component", "DocsHandler").
		Str("function", "RegisterRoutes").
		Logger()

	if router == nil {
		log.Error().Msg("Le routeur est nul, impossible d'enregistrer les routes de documentation")
		return
	}

	log.Debug().Msg("Enregistrement des routes de documentation")

	// Route pour la documentation utilisateur et administrateur
	router.GET("/docs/:type", h.DocsHandler)

	// Alias pour la documentation utilisateur
	router.GET("/docs", func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		log.Debug().Msg("Redirection vers la documentation utilisateur par défaut")
		h.DocsHandler(w, r, httprouter.Params{
			{Key: "type", Value: "user"},
		})
	})

	log.Info().
		Strs("routes", []string{"/docs", "/docs/:type"}).
		Msg("Routes de documentation enregistrées avec succès")
}
