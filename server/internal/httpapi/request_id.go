package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"pvmss/server/internal/logctx"
	"regexp"
)

const requestIDHeader = "X-Request-Id"

// validRequestID bounds an inbound ID to 1-64 safe chars so it can be echoed
// in a header and logged without injection (CRLF, spaces, path chars).
var validRequestID = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

// withRequestID gives every request an ID (inbound if valid, else 16 random
// bytes as hex), echoes it in the response, and stores a request-scoped logger
// carrying it in the context. Keep it outermost so every layer can use it.
func withRequestID(log *slog.Logger, next http.Handler) http.Handler {
	if log == nil {
		log = slog.Default()
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(requestIDHeader)
		if !validRequestID.MatchString(id) {
			id = newRequestID()
		}

		w.Header().Set(requestIDHeader, id)

		ctx := logctx.With(r.Context(), log.With("requestId", id))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func newRequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand does not fail on supported platforms; an empty ID is
		// still a valid (if useless) value and must not break the request.
		return "unknown"
	}

	return hex.EncodeToString(b[:])
}
