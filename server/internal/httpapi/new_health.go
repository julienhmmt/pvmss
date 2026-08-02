package httpapi

import "log/slog"

// NewHealth creates a health handler for the given store.
func NewHealth(store Pinger, log *slog.Logger) *Health {
	return &Health{store: store, log: log}
}
