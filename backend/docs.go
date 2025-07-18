package main

import (
	"fmt"
	"os"
	"net/http"
	"path/filepath"

	"github.com/gomarkdown/markdown"
	htmlrenderer "github.com/gomarkdown/markdown/html"
	htmltemplate "html/template"

	"github.com/rs/zerolog/log"
)

// serveDocHandler serves admin/user documentation as HTML, localized by lang param/cookie/header.
func serveDocHandler(docType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		lang := r.URL.Query().Get("lang")
		log.Info().Str("handler", "serveDocHandler").Str("docType", docType).Str("lang", lang).Str("method", r.Method).Str("path", r.URL.Path).Msg("Serving documentation request")
		if lang == "" {
			if c, err := r.Cookie("pvmss_lang"); err == nil {
				lang = c.Value
			}
		}
		if lang == "" {
			lang = "en"
		}

		// Build path to markdown file
		mdPath := filepath.Join("./docs", fmt.Sprintf("%s.%s.md", docType, lang))
		mdBytes, err := os.ReadFile(mdPath)
		if err != nil {
			log.Warn().Str("docType", docType).Str("lang", lang).Str("mdPath", mdPath).Msg("Translation missing, falling back to English")
			// fallback to English if translation missing
			mdPath = filepath.Join("./docs", fmt.Sprintf("%s.en.md", docType))
			mdBytes, err = os.ReadFile(mdPath)
			if err != nil {
				log.Error().Err(err).Str("docType", docType).Str("mdPath", mdPath).Msg("Failed to read documentation file")
				http.Error(w, "Documentation not found", http.StatusNotFound)
				return
			}
		}

		renderer := htmlrenderer.NewRenderer(htmlrenderer.RendererOptions{Flags: htmlrenderer.CommonFlags | htmlrenderer.HrefTargetBlank})
		htmlBytes := markdown.ToHTML(mdBytes, nil, renderer)
		htmlStr := fmt.Sprintf(`<section class="section"><div class="container content">%s</div></section>`, string(htmlBytes))

		data := map[string]interface{}{
			"Title":      docType + " documentation",
			"Content":    htmltemplate.HTML(htmlStr),
		}
		localizePage(w, r, data)
		// Render using the main layout template
		renderTemplate(w, r, "", data)
	}
}
