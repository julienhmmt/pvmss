package logger

import (
	cryptorand "crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	mathrand "math/rand"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Logger is the application-wide logger type, aliased to zerolog.Logger.
// This allows other packages to depend only on pvmss/logger instead of importing zerolog directly.
type Logger = zerolog.Logger

// ContextKey is the type for context keys used in logging.
type ContextKey string

const (
	// RequestIDKey is the context key for request ID.
	RequestIDKey ContextKey = "request_id"
	// CorrelationIDKey is the context key for correlation ID.
	CorrelationIDKey ContextKey = "correlation_id"
	// UserIDKey is the context key for user ID.
	UserIDKey ContextKey = "user_id"
)

// Sampler handles log sampling for high-volume logs.
type Sampler struct {
	mu             sync.Mutex
	sampleRates    map[string]int
	sampleCounters map[string]int
	lastReset      map[string]time.Time // Track last reset per message type
	resetInterval  time.Duration
}

// NewSampler creates a new log sampler.
func NewSampler(resetInterval time.Duration) *Sampler {
	s := &Sampler{
		sampleRates:    make(map[string]int),
		sampleCounters: make(map[string]int),
		lastReset:      make(map[string]time.Time),
		resetInterval:  resetInterval,
	}
	return s
}

// SetSampleRate sets the sampling rate for a log message type.
// rate = 1 means log every message, rate = 10 means log 1 out of 10 messages.
func (s *Sampler) SetSampleRate(messageType string, rate int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sampleRates[messageType] = rate
}

// ShouldSample determines if a message should be logged based on sampling rate.
func (s *Sampler) ShouldSample(messageType string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Reset counter for this message type if interval has passed
	lastReset, exists := s.lastReset[messageType]
	if !exists || time.Since(lastReset) > s.resetInterval {
		s.sampleCounters[messageType] = 0
		s.lastReset[messageType] = time.Now()
	}

	rate, ok := s.sampleRates[messageType]
	if !ok || rate <= 1 {
		return true // No sampling or rate is 1 (log all)
	}

	count := s.sampleCounters[messageType] + 1
	s.sampleCounters[messageType] = count
	return count%rate == 1
}

// Global sampler instance.
var globalSampler = NewSampler(1 * time.Minute)

// Global math/rand source seeded once at init for fallback ID generation.
var mathRandSource = mathrand.New(mathrand.NewSource(time.Now().UnixNano()))

// Atomic counter for fallback ID generation when crypto/rand fails.
var fallbackIDCounter uint64

// GetSampler returns the global sampler instance.
func GetSampler() *Sampler {
	return globalSampler
}

// StackTrace captures the current stack trace.
func StackTrace() string {
	const maxDepth = 64 // Increased from 32 for deeper call stacks
	pcs := make([]uintptr, maxDepth)
	n := runtime.Callers(3, pcs) // Skip 3 frames: runtime.Callers, StackTrace, caller
	if n == 0 {
		return ""
	}

	frames := runtime.CallersFrames(pcs[:n])
	var sb strings.Builder
	for {
		frame, more := frames.Next()
		fmt.Fprintf(&sb, "%s\n    %s:%d\n", frame.Function, frame.File, frame.Line)
		if !more {
			break
		}
	}
	return sb.String()
}

// Init initializes the logger with the specified log level
func Init(level string) {
	// Set time format
	zerolog.TimeFieldFormat = time.RFC3339Nano

	// Read logging configuration from environment
	outputMode := strings.ToLower(strings.TrimSpace(os.Getenv("LOG_OUTPUT")))
	if outputMode == "" {
		outputMode = "stdout"
	}

	format := strings.ToLower(strings.TrimSpace(os.Getenv("LOG_FORMAT")))
	if format == "" {
		format = "console"
	}

	logFilePath := strings.TrimSpace(os.Getenv("LOG_FILE_PATH"))

	stdoutEnabled := outputMode == "stdout" || outputMode == "both"
	fileEnabled := outputMode == "file" || outputMode == "both"

	writers := make([]io.Writer, 0, 2)
	deferredWarnings := make([]string, 0, 2)

	// Configure stdout writer
	if stdoutEnabled {
		if format == "json" {
			writers = append(writers, os.Stdout)
		} else {
			consoleWriter := zerolog.ConsoleWriter{
				Out:        os.Stdout,
				TimeFormat: "2006-01-02 15:04:05",
			}
			writers = append(writers, consoleWriter)
		}
	}

	// Configure file writer if requested
	if fileEnabled {
		if logFilePath == "" {
			deferredWarnings = append(deferredWarnings, "LOG_OUTPUT requires a file but LOG_FILE_PATH is not set; disabling file logging")
		} else {
			file, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
			if err != nil {
				deferredWarnings = append(deferredWarnings, fmt.Sprintf("Failed to open log file '%s', disabling file logging: %v", logFilePath, err))
			} else {
				if format == "json" {
					writers = append(writers, file)
				} else {
					consoleWriter := zerolog.ConsoleWriter{
						Out:        file,
						TimeFormat: "2006-01-02 15:04:05",
					}
					writers = append(writers, consoleWriter)
				}
			}
		}
	}

	// Fallback: if no writers configured, use stdout console
	if len(writers) == 0 {
		consoleWriter := zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: "2006-01-02 15:04:05",
		}
		writers = append(writers, consoleWriter)
		deferredWarnings = append(deferredWarnings, "No valid log output configured, falling back to stdout console")
		stdoutEnabled = true
		fileEnabled = false
		logFilePath = ""
	}

	var output io.Writer
	if len(writers) == 1 {
		output = writers[0]
	} else {
		output = zerolog.MultiLevelWriter(writers...)
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

	// Log any deferred warnings about configuration
	for _, msg := range deferredWarnings {
		log.Warn().Msg(msg)
	}

	log.Info().
		Str("level", zerolog.GlobalLevel().String()).
		Str("output_mode", outputMode).
		Str("format", format).
		Bool("stdout_enabled", stdoutEnabled).
		Bool("file_enabled", fileEnabled).
		Str("log_file_path", logFilePath).
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

// Event is an alias for zerolog.Event to allow building log entries without importing zerolog.
type Event = zerolog.Event

// --- Structured Event Helpers for SIEM-ready logging ---

// WithRequestID adds a request ID to the log event.
func WithRequestID(requestID string) *zerolog.Event {
	return log.Info().Str("request_id", requestID)
}

// WithCorrelationID adds a correlation ID to the log event.
func WithCorrelationID(correlationID string) *zerolog.Event {
	return log.Info().Str("correlation_id", correlationID)
}

// WithUser adds a user ID to the log event.
func WithUser(userID string) *zerolog.Event {
	return log.Info().Str("user_id", userID)
}

// WithContext adds request ID, correlation ID, and user ID to the log event.
func WithContext(requestID, correlationID, userID string) *zerolog.Event {
	event := log.Info()
	if requestID != "" {
		event = event.Str("request_id", requestID)
	}
	if correlationID != "" {
		event = event.Str("correlation_id", correlationID)
	}
	if userID != "" {
		event = event.Str("user_id", userID)
	}
	return event
}

// GenerateRequestID generates a new unique request ID.
// Falls back to math/rand if crypto/rand fails.
func GenerateRequestID() string {
	b := make([]byte, 16)
	if _, err := cryptorand.Read(b); err != nil {
		// Fallback to math/rand if crypto/rand fails
		// Use global seeded source to avoid duplicate IDs in rapid calls
		mathRandSource.Read(b)
		// Add atomic counter to ensure uniqueness even if seed is same
		counter := atomic.AddUint64(&fallbackIDCounter, 1)
		b[0] ^= byte(counter)
		b[1] ^= byte(counter >> 8)
		b[2] ^= byte(counter >> 16)
		b[3] ^= byte(counter >> 24)
	}
	return hex.EncodeToString(b)
}

// GenerateCorrelationID generates a new unique correlation ID.
// Falls back to math/rand if crypto/rand fails.
func GenerateCorrelationID() string {
	b := make([]byte, 16)
	if _, err := cryptorand.Read(b); err != nil {
		// Fallback to math/rand if crypto/rand fails
		// Use global seeded source to avoid duplicate IDs in rapid calls
		mathRandSource.Read(b)
		// Add atomic counter to ensure uniqueness even if seed is same
		counter := atomic.AddUint64(&fallbackIDCounter, 1)
		b[0] ^= byte(counter)
		b[1] ^= byte(counter >> 8)
		b[2] ^= byte(counter >> 16)
		b[3] ^= byte(counter >> 24)
	}
	return hex.EncodeToString(b)
}

// ErrorWithStack logs an error with stack trace.
func ErrorWithStack(err error) *zerolog.Event {
	return log.Error().
		Err(err).
		Str("stack_trace", StackTrace())
}

// ErrorWithStackAndContext logs an error with stack trace and context.
func ErrorWithStackAndContext(err error, requestID, correlationID, userID string) *zerolog.Event {
	event := log.Error().
		Err(err).
		Str("stack_trace", StackTrace())
	if requestID != "" {
		event = event.Str("request_id", requestID)
	}
	if correlationID != "" {
		event = event.Str("correlation_id", correlationID)
	}
	if userID != "" {
		event = event.Str("user_id", userID)
	}
	return event
}

// Sampled logs a message only if sampling allows it.
// Returns a no-op event if sampling is disabled for this message type.
func Sampled(messageType string, level zerolog.Level) *zerolog.Event {
	if !globalSampler.ShouldSample(messageType) {
		// Return a disabled event (no-op) instead of nil to prevent panics
		return log.WithLevel(zerolog.Disabled)
	}
	switch level {
	case zerolog.DebugLevel:
		return log.Debug().Str("sampled_type", messageType)
	case zerolog.InfoLevel:
		return log.Info().Str("sampled_type", messageType)
	case zerolog.WarnLevel:
		return log.Warn().Str("sampled_type", messageType)
	case zerolog.ErrorLevel:
		return log.Error().Str("sampled_type", messageType)
	default:
		return log.Info().Str("sampled_type", messageType)
	}
}

// AuthEvent logs authentication-related events with standardized fields.
// Use for login success/failure, logout, session events.
func AuthEvent(eventType string) *zerolog.Event {
	return log.Info().
		Str("event_category", "auth").
		Str("event_type", eventType)
}

// AuthFailure logs authentication failures with standardized fields.
func AuthFailure(eventType, reason string) *zerolog.Event {
	return log.Warn().
		Str("event_category", "auth").
		Str("event_type", eventType).
		Str("failure_reason", reason)
}

// VMEvent logs VM lifecycle events with standardized fields.
// Use for create, delete, start, stop, reboot actions.
func VMEvent(eventType string, vmid int, node string) *zerolog.Event {
	return log.Info().
		Str("event_category", "vm").
		Str("event_type", eventType).
		Int("vmid", vmid).
		Str("node", node)
}

// VMFailure logs VM operation failures with standardized fields.
func VMFailure(eventType string, vmid int, node, reason string) *zerolog.Event {
	return log.Error().
		Str("event_category", "vm").
		Str("event_type", eventType).
		Int("vmid", vmid).
		Str("node", node).
		Str("failure_reason", reason)
}

// AdminEvent logs admin actions for audit trail.
func AdminEvent(eventType, username string) *zerolog.Event {
	return log.Info().
		Str("event_category", "admin").
		Str("event_type", eventType).
		Str("admin_username", username)
}

// SecurityEvent logs security-related events (CSRF failures, rate limiting, etc.).
func SecurityEvent(eventType string) *zerolog.Event {
	return log.Warn().
		Str("event_category", "security").
		Str("event_type", eventType)
}

// ConsoleEvent logs VM console access events.
func ConsoleEvent(eventType string, vmid int, node string) *zerolog.Event {
	return log.Info().
		Str("event_category", "console").
		Str("event_type", eventType).
		Int("vmid", vmid).
		Str("node", node)
}

// ProxmoxEvent logs Proxmox connectivity and API events.
func ProxmoxEvent(eventType string) *zerolog.Event {
	return log.Info().
		Str("event_category", "proxmox").
		Str("event_type", eventType)
}

// ProxmoxFailure logs Proxmox-related failures.
func ProxmoxFailure(eventType, reason string) *zerolog.Event {
	return log.Error().
		Str("event_category", "proxmox").
		Str("event_type", eventType).
		Str("failure_reason", reason)
}

// APIEvent logs API request/response events.
func APIEvent(eventType string) *zerolog.Event {
	return log.Info().
		Str("event_category", "api").
		Str("event_type", eventType)
}

// APIFailure logs API-related failures.
func APIFailure(eventType, reason string) *zerolog.Event {
	return log.Error().
		Str("event_category", "api").
		Str("event_type", eventType).
		Str("failure_reason", reason)
}

// DatabaseEvent logs database-related events.
func DatabaseEvent(eventType string) *zerolog.Event {
	return log.Info().
		Str("event_category", "database").
		Str("event_type", eventType)
}

// DatabaseFailure logs database-related failures.
func DatabaseFailure(eventType, reason string) *zerolog.Event {
	return log.Error().
		Str("event_category", "database").
		Str("event_type", eventType).
		Str("failure_reason", reason)
}
