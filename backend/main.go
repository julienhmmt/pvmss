package main

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/joho/godotenv"
	"pvmss/backend/proxmox"
)

var templates *template.Template

func main() {
	if err := godotenv.Load("../.env"); err != nil {
		log.Warn().Msg("Error loading .env file, relying on environment variables")
	}

	// Log the loaded Proxmox URL to verify it's loaded correctly
	proxmoxURL := os.Getenv("PROXMOX_URL")
	if proxmoxURL == "" {
		log.Warn().Msg("PROXMOX_URL is not set")
	} else {
		log.Info().Str("PROXMOX_URL", proxmoxURL).Msg("Proxmox URL loaded")
	}

	// Initialize logger
	zerolog.TimeFieldFormat = time.RFC3339Nano
	log.Logger = log.Output(zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: "2006-01-02 15:04:05",
	})

	// Initialize i18n
	initI18n()

	// Parse templates
	funcMap := template.FuncMap{
		"div": func(a, b float64) float64 {
			if b == 0 {
				return 0
			}
			return a / b
		},
		"mul": func(a, b float64) float64 {
			return a * b
		},
		"humanBytes": func(b float64) string {
			bytes := int64(b)
			const unit = 1024
			if bytes < unit {
				return fmt.Sprintf("%d B", bytes)
			}
			div, exp := int64(unit), 0
			for n := bytes / unit; n >= unit; n /= unit {
				div *= unit
				exp++
			}
			return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
		},
		"formatMemory": formatMemory,
	}

	var err error
	templates, err = template.New("layout.html").Funcs(funcMap).ParseGlob("../frontend/*.html")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to parse templates")
	}

	// Get port from environment
	port := os.Getenv("PORT")
	if port == "" {
		port = "50000"
		log.Info().Msgf("Using default port: %s", port)
	}

	// Create router
	r := http.NewServeMux()

	// Serve static files
	fs := http.FileServer(http.Dir("../frontend/css"))
	r.Handle("/css/", http.StripPrefix("/css/", fs))

	// Handlers
	r.HandleFunc("/", indexHandler)
	r.HandleFunc("/search", searchHandler)
	r.HandleFunc("/admin", adminHandler)
	r.HandleFunc("/storage", storagePageHandler)
	r.HandleFunc("/iso", isoPageHandler)
	r.HandleFunc("/health", healthHandler)

	// API handlers
	r.HandleFunc("/api/tags", tagsHandler)
	r.HandleFunc("/api/tags/", tagsHandler)
	r.HandleFunc("/api/storage", storageHandler)
	r.HandleFunc("/api/iso/all", allIsosHandler)
	r.HandleFunc("/api/settings", settingsHandler)
	r.HandleFunc("/api/iso/settings", updateIsoSettingsHandler)

	// Configure server
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	// Graceful shutdown
	go func() {
		log.Info().Str("port", port).Msg("Starting server...")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("Server failed to start")
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info().Msg("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("Server forced to shutdown")
	}

	log.Info().Msg("Server exiting")
}

func formatMemory(mem interface{}) string {
	if mem == nil {
		return "N/A"
	}
	memFloat, ok := mem.(float64)
	if !ok {
		return "N/A"
	}

	memBytes := int64(memFloat)
	memMB := memBytes / 1024 / 1024

	if memMB > 1024 {
		memGB := float64(memBytes) / 1024 / 1024 / 1024
		return fmt.Sprintf("%.1f GB", memGB)
	}
	return fmt.Sprintf("%d MB", memMB)
}

func renderTemplate(w http.ResponseWriter, r *http.Request, name string, data map[string]interface{}) {
	localizePage(w, r, data)

	// Render the specific page template to a buffer
	buf := new(bytes.Buffer)
	if err := templates.ExecuteTemplate(buf, name, data); err != nil {
		log.Error().Err(err).Msgf("Error executing page template: %s", name)
		http.Error(w, "Could not execute page template", http.StatusInternalServerError)
		return
	}

	// Add the rendered content to the data map
	data["Content"] = template.HTML(buf.String())

	// Render the main layout with the page content
	if err := templates.ExecuteTemplate(w, "layout", data); err != nil {
		log.Error().Err(err).Msg("Error executing layout template")
		http.Error(w, "Could not execute layout template", http.StatusInternalServerError)
	}
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	log.Info().Str("path", r.URL.Path).Msg("Request received for index")
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	data := make(map[string]interface{})
	renderTemplate(w, r, "index.html", data)
}

func searchHandler(w http.ResponseWriter, r *http.Request) {
	log.Info().Str("path", r.URL.Path).Msg("Request received for search")
	data := make(map[string]interface{})

	if r.Method == http.MethodPost {
		r.ParseForm()
		vmid := r.FormValue("vmid")
		name := r.FormValue("name")

		log.Info().Str("vmid", vmid).Str("name", name).Msg("Processing search request")

		apiURL := os.Getenv("PROXMOX_URL")
		apiTokenID := os.Getenv("PROXMOX_API_TOKEN_NAME")
		apiTokenSecret := os.Getenv("PROXMOX_API_TOKEN_VALUE")
		insecure := os.Getenv("PROXMOX_VERIFY_SSL") == "false"

		client, err := proxmox.NewClient(apiURL, apiTokenID, apiTokenSecret, insecure)
		if err != nil {
			log.Error().Err(err).Msg("Failed to create proxmox client")
			data["Error"] = "Failed to connect to Proxmox API"
		} else {
			results, err := searchVM(client, vmid, name)
			if err != nil {
				log.Error().Err(err).Msg("Failed to search for VM")
				data["Error"] = "Failed to retrieve VM information from Proxmox"
			} else {
				data["Results"] = results
				if vmid != "" {
					data["Query"] = vmid
				} else {
					data["Query"] = name
				}
			}
		}
	}

	renderTemplate(w, r, "search.html", data)
}

func adminHandler(w http.ResponseWriter, r *http.Request) {
	log.Info().Str("path", r.URL.Path).Msg("Request received for admin")
	data := make(map[string]interface{})

	apiURL := os.Getenv("PROXMOX_URL")
	apiTokenID := os.Getenv("PROXMOX_API_TOKEN_NAME")
	apiTokenSecret := os.Getenv("PROXMOX_API_TOKEN_VALUE")
	insecure := os.Getenv("PROXMOX_VERIFY_SSL") == "false"

	client, err := proxmox.NewClient(apiURL, apiTokenID, apiTokenSecret, insecure)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create proxmox client")
		data["Error"] = "Failed to connect to Proxmox API"
	} else {
		nodesResult, err := proxmox.GetNodes(client)
		if err != nil {
			log.Error().Err(err).Msg("Failed to get node list")
			data["Error"] = "Failed to retrieve nodes from Proxmox"
		} else {
			if nodesMap, ok := nodesResult.(map[string]interface{}); ok {
				data["Nodes"] = nodesMap["data"]
			} else {
				log.Error().Msg("Failed to assert nodes to map[string]interface{}")
				data["Error"] = "Failed to process node data from Proxmox"
			}
		}
	}

	renderTemplate(w, r, "admin.html", data)
}

func storagePageHandler(w http.ResponseWriter, r *http.Request) {
	log.Info().Str("path", r.URL.Path).Msg("Request received for storage page")
	data := make(map[string]interface{})
	renderTemplate(w, r, "storage.html", data)
}

func isoPageHandler(w http.ResponseWriter, r *http.Request) {
	log.Info().Str("path", r.URL.Path).Msg("Request received for iso page")
	data := make(map[string]interface{})
	renderTemplate(w, r, "iso.html", data)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}