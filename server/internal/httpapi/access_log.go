package httpapi

import (
	"bufio"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"pvmss/server/internal/logctx"
)

const (
	routeHealth    = "/health"
	routeSPA       = "/"
	routeUnmatched = "unmatched"
	routeMe        = "/api/v1/auth/me"
)

// logWriter records what the access log needs (status, bytes, attached error,
// hijack) while staying transparent: Flush and Hijack are forwarded and
// Unwrap lets http.ResponseController reach the real writer.
type logWriter struct {
	http.ResponseWriter
	status   int
	bytes    int
	err      error
	hijacked bool
	onHijack func()
}

func (w *logWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}

	w.ResponseWriter.WriteHeader(code)
}

func (w *logWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}

	n, err := w.ResponseWriter.Write(p)
	w.bytes += n

	return n, err
}

func (w *logWriter) Flush() { _ = http.NewResponseController(w.ResponseWriter).Flush() }

func (w *logWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	conn, rw, err := http.NewResponseController(w.ResponseWriter).Hijack()
	if err == nil {
		w.hijacked = true
		w.status = http.StatusSwitchingProtocols

		w.onHijack()
	}

	return conn, rw, err
}

func (w *logWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// SetError attaches the cause of a failed response to the access-log wrapper
// without logging it; the access-log line carries it as "error". Handlers call
// it instead of logging an error they also turn into a response. It is a no-op
// when w is not (or does not wrap) the access-log writer.
func SetError(w http.ResponseWriter, err error) {
	for w != nil {
		if lw, ok := w.(*logWriter); ok {
			lw.err = err
			return
		}

		u, ok := w.(interface{ Unwrap() http.ResponseWriter })
		if !ok {
			return
		}

		w = u.Unwrap()
	}
}

// accessLevel maps a finished request to its log level. Health and the SPA
// shell are capped at Debug: they are noise, not application activity.
func accessLevel(method, route string, status int) slog.Level {
	if route == routeHealth || route == routeSPA {
		return slog.LevelDebug
	}

	switch {
	case status >= http.StatusInternalServerError:
		return slog.LevelError
	case status == http.StatusUnauthorized && route == routeMe:
		return slog.LevelDebug
	case status >= http.StatusBadRequest:
		return slog.LevelWarn
	case method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions:
		if status == http.StatusSwitchingProtocols {
			return slog.LevelInfo
		}

		return slog.LevelDebug
	default:
		return slog.LevelInfo
	}
}

// routeOf returns the mux pattern without its method prefix, never the URL.
func routeOf(r *http.Request) string {
	if r.Pattern == "" {
		return routeUnmatched
	}

	if _, path, ok := strings.Cut(r.Pattern, " "); ok {
		return path
	}

	return r.Pattern
}

// withAccessLog writes one line per request (two for a hijacked WebSocket:
// upgrade and close). Place it inside withRequestID so the line carries
// requestId and, once auth resolves, user.
func withAccessLog(trustedProxyHops int, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ip := clientIP(r, trustedProxyHops)

		emit := func(msg string, lw *logWriter) {
			route := routeOf(r)
			attrs := []slog.Attr{
				slog.String("method", r.Method),
				slog.String("route", route),
				slog.Int("status", lw.status),
				slog.Int("bytes", lw.bytes),
				slog.Int64("durationMs", time.Since(start).Milliseconds()),
				slog.String("clientIp", ip),
			}
			if lw.err != nil {
				attrs = append(attrs, slog.Any("error", lw.err))
			}

			logctx.From(r.Context()).LogAttrs(r.Context(), accessLevel(r.Method, route, lw.status), msg, attrs...)
		}

		lw := &logWriter{ResponseWriter: w}
		lw.onHijack = func() { emit("websocket upgraded", lw) }

		next.ServeHTTP(lw, r)

		if lw.status == 0 {
			lw.status = http.StatusOK
		}

		if lw.hijacked {
			emit("websocket closed", lw)
			return
		}

		emit("http request", lw)
	})
}
