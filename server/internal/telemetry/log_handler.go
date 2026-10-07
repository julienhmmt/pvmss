package telemetry

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

// logHandler adds traceId and spanId to every record logged with a context
// that carries a valid span. Calls without a context (plain Info, Warn, ...)
// get no ids, which is why sloglint's context rule matters.
type logHandler struct{ slog.Handler }

// LogHandler wraps next so records logged inside a span carry traceId/spanId.
func LogHandler(next slog.Handler) slog.Handler { return logHandler{next} }

func (h logHandler) Handle(ctx context.Context, r slog.Record) error {
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		r.AddAttrs(slog.String("traceId", sc.TraceID().String()), slog.String("spanId", sc.SpanID().String()))
	}

	return h.Handler.Handle(ctx, r)
}

func (h logHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return logHandler{h.Handler.WithAttrs(attrs)}
}

func (h logHandler) WithGroup(name string) slog.Handler {
	return logHandler{h.Handler.WithGroup(name)}
}
