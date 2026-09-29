// Package logctx carries a request-scoped *slog.Logger in a context.Context so
// any code holding the request ctx can log with requestId/user attached,
// without threading a logger through signatures.
package logctx

import (
	"context"
	"log/slog"
	"sync"
)

type ctxKey struct{}

// holder is stored once per request; AddAttrs mutates it so attrs added late
// (e.g. user after auth) are visible to everything sharing the ctx, including
// middleware that wrapped the request before the attr existed.
type holder struct {
	mu    sync.Mutex
	base  *slog.Logger
	attrs []any
}

// With returns a ctx carrying l. Call it once per request.
func With(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, &holder{base: l})
}

// From returns the request logger with every added attr, or slog.Default()
// when ctx carries none. It never returns nil.
func From(ctx context.Context) *slog.Logger {
	h, ok := ctx.Value(ctxKey{}).(*holder)
	if !ok {
		return slog.Default()
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	return h.base.With(h.attrs...)
}

// AddAttrs enriches the ctx logger. It is a no-op when ctx has no holder.
func AddAttrs(ctx context.Context, attrs ...slog.Attr) {
	h, ok := ctx.Value(ctxKey{}).(*holder)
	if !ok {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	for _, a := range attrs {
		h.attrs = append(h.attrs, a)
	}
}
