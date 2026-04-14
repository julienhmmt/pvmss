package logger

import (
	"context"
	"net/http"

	"github.com/rs/zerolog/log"
)

// RequestIDMiddleware is a middleware that injects a request ID into the request context.
// It generates a new request ID for each incoming request and stores it in the context.
// The request ID can then be accessed using the RequestIDKey context key.
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = GenerateRequestID()
		}

		ctx := context.WithValue(r.Context(), RequestIDKey, requestID)
		r = r.WithContext(ctx)

		w.Header().Set("X-Request-ID", requestID)

		next.ServeHTTP(w, r)
	})
}

// CorrelationIDMiddleware is a middleware that injects a correlation ID into the request context.
// It uses the X-Correlation-ID header if present, otherwise generates a new one.
// The correlation ID can be used to track related requests across services.
func CorrelationIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		correlationID := r.Header.Get("X-Correlation-ID")
		if correlationID == "" {
			correlationID = GenerateCorrelationID()
		}

		ctx := context.WithValue(r.Context(), CorrelationIDKey, correlationID)
		r = r.WithContext(ctx)

		w.Header().Set("X-Correlation-ID", correlationID)

		next.ServeHTTP(w, r)
	})
}

// LoggingMiddleware is a middleware that logs HTTP requests and responses.
// It includes request ID, correlation ID, and other relevant information.
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Type assertion with blank identifier is acceptable here:
		// If the value is not a string, we get empty string which is safe for logging
		requestID, _ := r.Context().Value(RequestIDKey).(string)
		correlationID, _ := r.Context().Value(CorrelationIDKey).(string)

		event := log.Debug()
		if requestID != "" {
			event = event.Str("request_id", requestID)
		}
		if correlationID != "" {
			event = event.Str("correlation_id", correlationID)
		}

		event.
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Str("remote_addr", r.RemoteAddr).
			Str("user_agent", r.UserAgent()).
			Msg("HTTP request")

		next.ServeHTTP(w, r)
	})
}

// GetRequestID extracts the request ID from the context.
// Returns empty string if not found.
func GetRequestID(ctx context.Context) string {
	if requestID, ok := ctx.Value(RequestIDKey).(string); ok {
		return requestID
	}
	return ""
}

// GetCorrelationID extracts the correlation ID from the context.
// Returns empty string if not found.
func GetCorrelationID(ctx context.Context) string {
	if correlationID, ok := ctx.Value(CorrelationIDKey).(string); ok {
		return correlationID
	}
	return ""
}

// GetUserID extracts the user ID from the context.
// Returns empty string if not found.
func GetUserID(ctx context.Context) string {
	if userID, ok := ctx.Value(UserIDKey).(string); ok {
		return userID
	}
	return ""
}
