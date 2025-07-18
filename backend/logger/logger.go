package logger

import (
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Init initializes the logger with the specified log level
func Init(level string) {
	// Set time format
	zerolog.TimeFieldFormat = time.RFC3339Nano
	
	// Configure console writer for human-friendly output
	output := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: "2006-01-02 15:04:05",
	}

	// Set the global logger
	log.Logger = log.Output(output)

	// Set log level based on environment variable
	switch strings.ToLower(level) {
	case "trace":
		zerolog.SetGlobalLevel(zerolog.TraceLevel)
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "warn":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	case "fatal":
		zerolog.SetGlobalLevel(zerolog.FatalLevel)
	case "panic":
		zerolog.SetGlobalLevel(zerolog.PanicLevel)
	case "info", "":
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	log.Info().
		Str("level", zerolog.GlobalLevel().String()).
		Msg("Logger initialized")
}

// Get returns a pointer to the configured logger instance
func Get() *zerolog.Logger {
	return &log.Logger
}
