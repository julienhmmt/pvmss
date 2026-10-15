// Package logctx carries a request-scoped *slog.Logger in a context.Context so
// any code holding the request ctx can log with requestId/user attached,
// without threading a logger through signatures.
package logctx

import (
	"context"
	"log/slog"
	"slices"
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
	return FromOr(ctx, slog.Default())
}

// FromOr is From with a caller-supplied fallback, for components that own an
// injected logger (e.g. the store) but should still use the request logger
// when one is present.
func FromOr(ctx context.Context, fallback *slog.Logger) *slog.Logger {
	h, ok := ctx.Value(ctxKey{}).(*holder)
	if !ok {
		return fallback
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	return h.base.With(h.attrs...)
}

// AddAttrs enriches the ctx logger; an attr whose key is already set
// replaces it (auth resolves the principal several times per request). It is
// a no-op when ctx has no holder.
func AddAttrs(ctx context.Context, attrs ...slog.Attr) {
	h, ok := ctx.Value(ctxKey{}).(*holder)
	if !ok {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	for _, a := range attrs {
		h.attrs = slices.DeleteFunc(h.attrs, func(old any) bool {
			existing, ok := old.(slog.Attr)
			return ok && existing.Key == a.Key
		})
		h.attrs = append(h.attrs, a)
	}
}
