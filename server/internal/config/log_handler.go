package config

import (
	"context"
	"log/slog"
	"regexp"
)

const redactedValue = "[redacted]"

// secretKey matches attribute keys whose values must never reach a log sink.
var secretKey = regexp.MustCompile(`(?i)password|token|secret|cookie|authorization|ticket|csrf`)

// redactSecrets masks the value of any leaf attribute whose key looks secret.
// slog calls ReplaceAttr for leaf attrs inside groups too, so nesting is covered.
func redactSecrets(_ []string, a slog.Attr) slog.Attr {
	if secretKey.MatchString(a.Key) {
		return slog.String(a.Key, redactedValue)
	}

	return a
}

// sourceGate drops the source location from records unless the live level is
// debug, so AddSource follows runtime level changes instead of being fixed at
// construction.
type sourceGate struct {
	slog.Handler
	level *slog.LevelVar
}

func (h sourceGate) Handle(ctx context.Context, r slog.Record) error {
	if h.level.Level() > slog.LevelDebug {
		r.PC = 0 // a zero PC makes the inner handler omit "source"
	}

	return h.Handler.Handle(ctx, r)
}

func (h sourceGate) WithAttrs(attrs []slog.Attr) slog.Handler {
	return sourceGate{Handler: h.Handler.WithAttrs(attrs), level: h.level}
}

func (h sourceGate) WithGroup(name string) slog.Handler {
	return sourceGate{Handler: h.Handler.WithGroup(name), level: h.level}
}
