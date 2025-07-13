package main

import (
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

func main() {
	// Initialize logger
	zerolog.TimeFieldFormat = time.RFC3339Nano
	log.Logger = log.Output(zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: "2006-01-02 15:04:05",
	})

	// Initialize i18n
	bundle := i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)
	bundle.MustLoadMessageFile("i18n/active.en.toml")

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

	// Root handler
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Info().Str("path", r.URL.Path).Msg("Request received")

		// Set up localizer
		lang := r.Header.Get("Accept-Language")
		localizer := i18n.NewLocalizer(bundle, lang)

		// Localize messages
		title := localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Title"})
		header := localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Header"})
		subtitle := localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Subtitle"})
		body := localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Body"})
		footer := localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Footer"})

		// Parse and execute template
		tmpl, err := template.ParseFiles("../frontend/index.html")
		if err != nil {
			http.Error(w, "Could not parse template", http.StatusInternalServerError)
			log.Error().Err(err).Msg("Template parsing error")
			return
		}

		data := map[string]string{
			"Title":    title,
			"Header":   header,
			"Subtitle": subtitle,
			"Body":     body,
			"Footer":   footer,
		}

		if err := tmpl.Execute(w, data); err != nil {
			http.Error(w, "Could not execute template", http.StatusInternalServerError)
			log.Error().Err(err).Msg("Template execution error")
		}
	})

	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

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