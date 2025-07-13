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
	"github.com/rs/zerolog/log"
	"golang.org/x/text/language"
)

var (
	templates *template.Template
	bundle    *i18n.Bundle
)

func main() {
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
	var err error
	templates, err = template.ParseGlob("../frontend/*.html")
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

	data["Title"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Title"})
	data["Header"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Header"})
	data["Subtitle"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Subtitle"})
	data["Body"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Body"})
	data["Footer"] = template.HTML(localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Footer"}))
	data["NavbarHome"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Navbar.Home"})
	data["NavbarVMs"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Navbar.VMs"})
	data["NavbarSearchVM"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Navbar.SearchVM"})
	data["ButtonSearchVM"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Button.Search"})
	data["SearchResults"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Search.Results"})
	data["SearchYouSearchedFor"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Search.YouSearchedFor"})
	data["NavbarAdmin"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Navbar.Admin"})
	data["Lang"] = lang

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
	query := r.URL.Query().Get("q")
	data := map[string]interface{}{
		"Query": query,
	}
	renderTemplate(w, r, "search.html", data)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}