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
	docsDir, err := findDocsDir()
	if err != nil {
		logger.Get().Error().Err(err).Msg("Failed to find docs directory")
	}

	return &DocsHandler{
		docsDir: docsDir,
	}
}

// DocsHandler gère les requêtes vers la documentation
func (h *DocsHandler) DocsHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// Déterminer la langue (par défaut: en)
	lang := r.URL.Query().Get("lang")
	if lang == "" {
		lang = "en" // Langue par défaut
	}

	// Déterminer le type de documentation (admin ou user)
	docType := ps.ByName("type")
	if docType == "" {
		docType = "user" // Type par défaut
	}

	// Construire le chemin du fichier de documentation
	docFile := filepath.Join(h.docsDir, fmt.Sprintf("%s.%s.md", docType, lang))

	// Vérifier si le fichier existe
	if _, err := os.Stat(docFile); os.IsNotExist(err) {
		// Essayer avec la langue par défaut si le fichier n'existe pas
		if lang != "en" {
			docFile = filepath.Join(h.docsDir, fmt.Sprintf("%s.en.md", docType))
		}

		// Vérifier à nouveau après le changement de langue
		if _, err := os.Stat(docFile); os.IsNotExist(err) {
			http.NotFound(w, r)
			return
		}
	}

	// Lire le contenu du fichier Markdown
	content, err := os.ReadFile(docFile)
	if err != nil {
		logger.Get().Error().Err(err).Str("file", docFile).Msg("Failed to read docs file")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Convertir le Markdown en HTML
	htmlContent := markdown.ToHTML(content, nil, nil)

	// Préparer les données pour le template
	data := map[string]interface{}{
		"Content":     template.HTML(htmlContent),
		"CurrentLang": lang,
		"DocType":     docType,
	}

	// Charger les traductions
	i18n.LocalizePage(w, r, data)
	data["Title"] = data["Docs.Title"]

	renderTemplateInternal(w, r, "docs", data)
}

// findDocsDir cherche le répertoire de documentation
func findDocsDir() (string, error) {
	// Liste des emplacements possibles pour le répertoire de documentation
	possibleDirs := []string{
		"./docs",
		"../docs",
		"./backend/docs",
		"/app/backend/docs",
		"/app/docs",
	}

	// Ajouter le répertoire de l'exécutable
	exePath, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)
		possibleDirs = append(possibleDirs, filepath.Join(exeDir, "docs"))
	}

	// Ajouter le répertoire du fichier source
	_, filename, _, ok := runtime.Caller(0)
	if ok {
		srcDir := filepath.Dir(filepath.Dir(filename))
		possibleDirs = append(possibleDirs, filepath.Join(srcDir, "docs"))
	}

	// Vérifier chaque emplacement possible
	for _, dir := range possibleDirs {
		if _, err := os.Stat(dir); err == nil {
			return dir, nil
		}
	}

	// Si aucun répertoire valide n'a été trouvé
	return "", fmt.Errorf("impossible de trouver le répertoire de documentation dans les emplacements suivants: %v", possibleDirs)
}

// RegisterRoutes enregistre les routes de documentation
func (h *DocsHandler) RegisterRoutes(router *httprouter.Router) {
	// Documentation utilisateur
	router.GET("/docs/user", func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		h.DocsHandler(w, r, httprouter.Params{
			{Key: "type", Value: "user"},
		})
	})

	// Documentation administrateur
	router.GET("/docs/admin", HandlerFuncToHTTPrHandle(RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		h.DocsHandler(w, r, httprouter.Params{
			{Key: "type", Value: "admin"},
		})
	})))
}
