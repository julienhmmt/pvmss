package handlers

import (
	"net/http"

	"pvmss/logger"
	"pvmss/state"
)

// ISOInfo represents detailed information about an ISO image.
type ISOInfo struct {
	VolID   string `json:"volid"`
	Format  string `json:"format"`
	Size    int64  `json:"size"`
	Node    string `json:"node,omitempty"`
	Storage string `json:"storage,omitempty"`
	Enabled bool   `json:"enabled"`
}

// handlerContextKey is used for context keys specific to handlers package
type handlerContextKey string

// ParamsKey is the key used to store httprouter.Params in the request context
const ParamsKey handlerContextKey = "params"

// StateManagerKey stores the state manager in request context
const StateManagerKey handlerContextKey = "stateManager"

// getStateManager returns the state manager from request context when available.
// No global fallback: state is injected by handlers.InitHandlers.
func getStateManager(r *http.Request) state.StateManager {
	if sm, ok := r.Context().Value(StateManagerKey).(state.StateManager); ok && sm != nil {
		return sm
	}
	logger.Get().Error().Msg("State manager missing from request context")
	return nil
}

// setNoCacheHeaders sets headers to prevent client-side caching.
func setNoCacheHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
}
