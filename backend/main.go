package main

import (
	"bytes"
	"context"
	"html/template"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/rs/zerolog"
	"fmt"
	"github.com/rs/zerolog/log"
	"golang.org/x/text/language"

	"github.com/joho/godotenv"
	"pvmss/backend/proxmox"
)

var (
	templates *template.Template
	bundle    *i18n.Bundle
)

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
	bundle = i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)
	bundle.MustLoadMessageFile("i18n/active.en.toml")
	bundle.MustLoadMessageFile("i18n/active.fr.toml")

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
	r.HandleFunc("/health", healthHandler)

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
	// Determine language from query param, cookie, or header
	lang := r.URL.Query().Get("lang")
	if lang != "" {
		// Set cookie if lang is from query param
		http.SetCookie(w, &http.Cookie{
			Name:    "pvmss_lang",
			Value:   lang,
			Path:    "/",
			Expires: time.Now().Add(365 * 24 * time.Hour),
		})
	} else {
		// Try to get lang from cookie
		cookie, err := r.Cookie("pvmss_lang")
		if err == nil {
			lang = cookie.Value
		}
	}

	// Fallback to header if no other lang source is found
	if lang == "" {
		lang = r.Header.Get("Accept-Language")
	}

	localizer := i18n.NewLocalizer(bundle, lang)

	data["AdminNodes"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Admin.Nodes"})
	data["AdminPage"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Admin.Page"})
	data["Body"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Body"})
	data["ButtonSearchVM"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Button.Search"})
	data["Footer"] = template.HTML(localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Footer"}))
	data["Header"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Header"})
	data["Lang"] = lang
	data["NavbarAdmin"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Navbar.Admin"})
	data["NavbarHome"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Navbar.Home"})
	data["NavbarSearchVM"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Navbar.SearchVM"})
	data["NavbarVMs"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Navbar.VMs"})
	data["SearchTitle"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Search.Title"})
	data["SearchVMID"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Search.VMID"})
	data["SearchName"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Search.Name"})
	data["SearchStatus"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Search.Status"})
	data["SearchCPUs"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Search.CPUs"})
	data["SearchMemory"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Search.Memory"})
	data["SearchResults"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Search.Results"})
	data["SearchYouSearchedFor"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Search.YouSearchedFor"})
	data["Subtitle"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Subtitle"})
	data["Title"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Title"})

	// Nodes page
	data["NodesNoNodes"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Nodes.NoNodes"})
	data["NodesHeaderNode"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Nodes.Header.Node"})
	data["NodesHeaderStatus"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Nodes.Header.Status"})
	data["NodesHeaderCPUUsage"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Nodes.Header.CPUUsage"})
	data["NodesHeaderMemoryUsage"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Nodes.Header.MemoryUsage"})
	data["NodesHeaderDiskUsage"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Nodes.Header.DiskUsage"})
	data["NodesStatusOnline"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Nodes.Status.Online"})
	data["NodesStatusOffline"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Nodes.Status.Offline"})

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

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}