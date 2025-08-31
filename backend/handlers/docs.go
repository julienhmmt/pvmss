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

// DocsHandler handles documentation routes
type DocsHandler struct {
	docsDir string
}

// NewDocsHandler creates a new instance of DocsHandler
func NewDocsHandler() *DocsHandler {
	log := logger.Get().With().
		Str("component", "DocsHandler").
		Str("function", "NewDocsHandler").
		Logger()

	log.Debug().Msg("Searching for documentation directory")

	docsDir, err := findDocsDir()
	if err != nil {
		log.Error().
			Err(err).
			Msg("Failed to find documentation directory")
	} else {
		log.Info().
			Str("docs_dir", docsDir).
			Msg("Successfully found documentation directory")
	}

	return &DocsHandler{
		docsDir: docsDir,
	}
}

// DocsHandler handles requests for documentation
func (h *DocsHandler) DocsHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// Create a logger for this request
	log := logger.Get().With().
		Str("handler", "DocsHandler").
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("remote_addr", r.RemoteAddr).
		Logger()

	log.Debug().Msg("Processing documentation request")

	// Check if the documentation directory is available
	if h.docsDir == "" {
		errMsg := "Documentation directory is not configured"
		log.Error().Msg(errMsg)
		http.Error(w, "Documentation not available", http.StatusServiceUnavailable)
		return
	}

	// Determine the language (default: en)
	lang := r.URL.Query().Get("lang")
	if lang == "" {
		lang = "en"
		log.Debug().Msg("No language specified, using English by default")
	} else {
		log.Debug().Str("lang", lang).Msg("Language specified in request")
	}

	// Determine the documentation type (admin or user)
	docType := ps.ByName("type")
	if docType == "" {
		docType = "user"
		log.Debug().Msg("No documentation type specified, using 'user' by default")
	} else {
		log.Debug().Str("doc_type", docType).Msg("Documentation type specified")
	}

	// Build the path to the documentation file
	docFile := filepath.Join(h.docsDir, fmt.Sprintf("%s.%s.md", docType, lang))

	log.Debug().
		Str("requested_file", docFile).
		Msg("Looking for documentation file")

	// Check if the file exists
	if _, err := os.Stat(docFile); os.IsNotExist(err) {
		log.Debug().
			Str("file", docFile).
			Msg("Documentation file not found, trying with English language")

		// Try with the default language if the file does not exist
		if lang != "en" {
			docFile = filepath.Join(h.docsDir, fmt.Sprintf("%s.en.md", docType))
			log.Debug().
				Str("fallback_file", docFile).
				Msg("Trying with the English fallback file")

			// Check again after language change
			if _, err := os.Stat(docFile); os.IsNotExist(err) {
				log.Warn().
					Str("file", docFile).
					Str("doc_type", docType).
					Str("lang", "en").
					Msg("Documentation file not found, even in English")

				http.NotFound(w, r)
				return
			}
		} else {
			log.Warn().
				Str("file", docFile).
				Msg("Documentation file not found and no alternative available")

			http.NotFound(w, r)
			return
		}
	}

	// Read the Markdown file content
	log.Debug().
		Str("file", docFile).
		Msg("Reading documentation file")

	content, err := os.ReadFile(docFile)
	if err != nil {
		log.Error().
			Err(err).
			Str("file", docFile).
			Msg("Failed to read documentation file")

		http.Error(w, "Internal server error while reading documentation", http.StatusInternalServerError)
		return
	}

	log.Debug().
		Int("file_size_bytes", len(content)).
		Msg("Successfully read documentation file content")

	// Convert Markdown to HTML
	htmlContent := markdown.ToHTML(content, nil, nil)

	log.Debug().
		Int("html_size_bytes", len(htmlContent)).
		Msg("Successfully converted Markdown to HTML")

	// Prepare data for the template
	data := map[string]interface{}{
		"Content":     template.HTML(htmlContent),
		"CurrentLang": lang,
		"DocType":     docType,
	}

	log.Debug().
		Str("doc_type", docType).
		Str("lang", lang).
		Msg("Preparing data for template rendering")

	// Load translations
	i18n.LocalizePage(w, r, data)
	data["Title"] = data["Docs.Title"]

	log.Debug().Msg("Calling documentation template renderer")
	renderTemplateInternal(w, r, "docs", data)

	log.Info().
		Str("doc_type", docType).
		Str("lang", lang).
		Msg("Successfully displayed documentation")
}

// findDocsDir searches for the documentation directory
func findDocsDir() (string, error) {
	log := logger.Get().With().
		Str("function", "findDocsDir").
		Logger()

	log.Debug().Msg("Searching for documentation directory")

	// Get the path of the calling source file
	_, filename, _, ok := runtime.Caller(0)
	if ok {
		log.Debug().
			Str("caller_file", filename).
			Msg("Identified caller file")
	}

	// List of possible locations for the documentation directory
	possibleDirs := []string{
		"./docs",
		"../docs",
		"./backend/docs",
		"/app/backend/docs",
		"/app/docs",
	}

	// Add the directory of the running binary
	execDir, err := os.Executable()
	if err == nil {
		execDir = filepath.Dir(execDir)
		docsPath := filepath.Join(execDir, "docs")
		possibleDirs = append(possibleDirs, docsPath)
		log.Debug().
			Str("exec_dir", execDir).
			Str("docs_path", docsPath).
			Msg("Execution directory added to search locations")
	} else {
		log.Warn().
			Err(err).
			Msg("Could not determine execution directory")
	}

	// Add the source file directory
	_, filename, _, ok = runtime.Caller(0)
	if ok {
		srcDir := filepath.Dir(filepath.Dir(filename))
		possibleDirs = append(possibleDirs, filepath.Join(srcDir, "docs"))
	}

	// Check each possible location
	for _, dir := range possibleDirs {
		log.Debug().
			Str("checking_dir", dir).
			Msg("Checking documentation location")

		info, err := os.Stat(dir)
		if err == nil && info.IsDir() {
			log.Info().
				Str("found_dir", dir).
				Msg("Documentation directory found")

			// Check that the directory contains documentation files
			entries, err := os.ReadDir(dir)
			if err == nil && len(entries) > 0 {
				log.Info().
					Str("dir", dir).
					Int("file_count", len(entries)).
					Msg("Valid documentation directory with files found")
				return dir, nil
			}

			log.Warn().
				Str("dir", dir).
				Msg("Directory found but empty or inaccessible")
		}
	}

	// If no valid directory was found
	return "", fmt.Errorf("could not find documentation directory in the following locations: %v", possibleDirs)
}

// RegisterRoutes registers documentation routes
func (h *DocsHandler) RegisterRoutes(router *httprouter.Router) {
	log := logger.Get().With().
		Str("component", "DocsHandler").
		Str("function", "RegisterRoutes").
		Logger()

	if router == nil {
		log.Error().Msg("Router is nil, cannot register documentation routes")
		return
	}

	log.Debug().Msg("Registering documentation routes")

	// Route for user and admin documentation
	router.GET("/docs/:type", h.DocsHandler)

	// Alias for user documentation
	router.GET("/docs", func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		log.Debug().Msg("Redirecting to default user documentation")
		h.DocsHandler(w, r, httprouter.Params{
			{Key: "type", Value: "user"},
		})
	})

	log.Info().
		Strs("routes", []string{"/docs", "/docs/:type"}).
		Msg("Documentation routes registered successfully")
}
