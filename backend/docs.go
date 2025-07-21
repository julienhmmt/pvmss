package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/gomarkdown/markdown"
	htmlrenderer "github.com/gomarkdown/markdown/html"
	htmltemplate "html/template"

	"github.com/rs/zerolog/log"
)

// serveDocHandler serves admin/user documentation as HTML
func serveDocHandler(docType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Initialize data map with common values
		data := make(map[string]interface{})
		
		// Set default values for template
		currentURL := r.URL.Path

		// Get the absolute path to the docs directory
		_, filename, _, _ := runtime.Caller(0)
		dir := filepath.Dir(filename)
		docsDir := filepath.Join(dir, "docs")

		// Build path to markdown file (English only for now)
		mdPath := filepath.Join(docsDir, fmt.Sprintf("%s.md", docType))
		mdBytes, err := os.ReadFile(mdPath)
		if err != nil {
			log.Error().
				Err(err).
				Str("docType", docType).
				Str("path", mdPath).
				Msg("Failed to read documentation file")
			http.Error(w, "Documentation not found", http.StatusNotFound)
			return
		}

		// Convert markdown to HTML
		renderer := htmlrenderer.NewRenderer(htmlrenderer.RendererOptions{
			Flags: htmlrenderer.CommonFlags | htmlrenderer.HrefTargetBlank,
		})
		htmlBytes := markdown.ToHTML(mdBytes, nil, renderer)
		htmlStr := string(htmlBytes)

		// Get title and description
		title := fmt.Sprintf("%s Documentation", strings.Title(docType))
		description := fmt.Sprintf("Documentation for %s features and configuration", docType)

		// Prepare template data
		data["Title"] = title
		data["Description"] = description
		data["Content"] = htmltemplate.HTML(htmlStr)
		data["CurrentURL"] = currentURL
		data["LangEN"] = currentURL
		data["LangFR"] = currentURL

		// Add navigation items
		data["NavbarHome"] = "Home"
		data["NavbarVMs"] = "Create VM"
		data["NavbarSearchVM"] = "Search VMs"
		data["NavbarUserDocs"] = "User Guide"
		data["NavbarAdmin"] = "Admin"

		// Add authentication data if user is logged in
		if token, ok := r.Context().Value("token").(string); ok && token != "" {
			data["IsAuthenticated"] = true
			if username, ok := r.Context().Value("username").(string); ok {
				data["Username"] = username
			}
		}

		// Render using the main layout template
		err = templates.ExecuteTemplate(w, "layout", data)
		if err != nil {
			log.Error().Err(err).Msg("Failed to execute template")
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}
}
