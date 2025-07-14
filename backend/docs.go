package main

import (
	"fmt"
	"os"
	"net/http"
	"path/filepath"

	"github.com/gomarkdown/markdown"
	htmlrenderer "github.com/gomarkdown/markdown/html"
	htmltemplate "html/template"
)

// serveDocHandler serves admin/user documentation as HTML, localized by lang param/cookie/header.
func serveDocHandler(docType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		lang := r.URL.Query().Get("lang")
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
			// fallback to English if translation missing
			mdPath = filepath.Join("./docs", fmt.Sprintf("%s.en.md", docType))
			mdBytes, err = os.ReadFile(mdPath)
			if err != nil {
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

func registerDocRoutes() {
	http.HandleFunc("/docs/admin", serveDocHandler("admin"))
	http.HandleFunc("/docs/user", serveDocHandler("user"))
}
