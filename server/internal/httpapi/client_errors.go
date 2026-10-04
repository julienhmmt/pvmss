package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"pvmss/server/internal/logctx"
)

// Bounds for the client-error report payload and its per-IP rate limit. The
// cap is generous - an error burst is exactly what this endpoint exists to
// see - but unbounded logging is a spam vector.
const (
	clientErrorBodyLimit    = 8 << 10
	clientErrorMaxMessage   = 512
	clientErrorMaxStack     = 4096
	clientErrorMaxPath      = 256
	clientErrorEventAttr    = "client_error"
	clientErrorEmptyMessage = "(empty message)"
)

// clientErrorRequest is what the SPA posts: the error it caught, trimmed to
// what the log may honestly carry. The client strips origin and query from
// the page URL - raw URLs are never logged (logging rules).
type clientErrorRequest struct {
	Message string `json:"message"`
	Stack   string `json:"stack,omitempty"`
	Path    string `json:"path,omitempty"`
}

// ServeClientError handles POST /api/v1/client-errors. The SPA forwards the
// errors it cannot report anywhere else (window.onerror, unhandled
// rejections, the SvelteKit handleError hook) so frontend failures reach the
// server log. It is unauthenticated - a pre-login error must still report -
// and per-IP rate limited. Unknown JSON fields are ignored so the client can
// grow its payload without a server deploy.
func ServeClientError(w http.ResponseWriter, r *http.Request) {
	var req clientErrorRequest

	defer func() { _ = r.Body.Close() }()

	// Tolerant decode (no DisallowUnknownFields): telemetry must keep working
	// when the client ships new fields before the server does.
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, clientErrorBodyLimit))
	if err := decoder.Decode(&req); err != nil {
		writeAuthError(w, http.StatusBadRequest, "invalid_request", "invalid client error payload")
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeAuthError(w, http.StatusBadRequest, "invalid_request", "invalid client error payload")
		return
	}

	message := truncateRunes(req.Message, clientErrorMaxMessage)
	if message == "" {
		message = clientErrorEmptyMessage
	}

	attrs := []any{
		"component", "httpapi",
		"event", clientErrorEventAttr,
		"error", message,
	}
	if path := truncateRunes(req.Path, clientErrorMaxPath); path != "" {
		attrs = append(attrs, "path", path)
	}
	if stack := truncateRunes(req.Stack, clientErrorMaxStack); stack != "" {
		attrs = append(attrs, "stack", stack)
	}

	logctx.From(r.Context()).WarnContext(r.Context(), "client error reported", attrs...)

	w.WriteHeader(http.StatusNoContent)
}

// truncateRunes cuts s at max runes so a hostile or buggy client cannot
// smuggle oversized values into a log line.
func truncateRunes(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}

	return string(runes[:max])
}
