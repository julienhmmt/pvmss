package logger

import (
	"io"
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

	// Set log level, defaulting to InfoLevel if parsing fails.
	lvl, err := zerolog.ParseLevel(strings.ToLower(level))
	if err != nil {
		lvl = zerolog.InfoLevel
		log.Warn().Str("log_level_in", level).Msg("Invalid log level, defaulting to 'info'")
	}
	zerolog.SetGlobalLevel(lvl)

	log.Info().
		Str("level", zerolog.GlobalLevel().String()).
		Msg("Logger initialized")
}

// Get returns a pointer to the configured logger instance
func Get() *zerolog.Logger {
	return &log.Logger
}

// SetOutput changes the destination for log output.
// This is useful for redirecting logs to a file or a buffer during testing.
func SetOutput(w io.Writer) {
	log.Logger = log.Output(w)
}
