package httpapi

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"
)

// logLevelDTO is the body of GET/PUT /api/v1/admin/ops/log-level. Default is
// the level the process started with (LOG_LEVEL); a restart returns to it.
type logLevelDTO struct {
	Level   string `json:"level"`
	Default string `json:"default"`
}

type logLevelRequest struct {
	Level string `json:"level"`
}

// SetLogLevel hands the handler the logger's LevelVar and the startup default.
// Until it is called the log-level routes are not registered.
func (h *AdminOps) SetLogLevel(level *slog.LevelVar, def slog.Level) {
	h.logLevel = level
	h.logLevelDefault = def
}

func levelName(l slog.Level) string { return strings.ToLower(l.String()) }

// parseLevelName accepts exactly the four names LOG_LEVEL accepts.
func parseLevelName(name string) (slog.Level, bool) {
	switch strings.ToLower(name) {
	case "debug":
		return slog.LevelDebug, true
	case "info":
		return slog.LevelInfo, true
	case "warn":
		return slog.LevelWarn, true
	case "error":
		return slog.LevelError, true
	}

	return slog.LevelInfo, false
}

func (h *AdminOps) logLevelState() logLevelDTO {
	return logLevelDTO{Level: levelName(h.logLevel.Level()), Default: levelName(h.logLevelDefault)}
}

// ServeLogLevel handles GET /api/v1/admin/ops/log-level.
func (h *AdminOps) ServeLogLevel(w http.ResponseWriter, _ *http.Request) {
	writeAdminJSON(w, http.StatusOK, h.logLevelState())
}

// ServeLogLevelUpdate handles PUT /api/v1/admin/ops/log-level. It changes the
// live LevelVar (filtering and source emission follow at once) and is audited;
// nothing is persisted, so a restart falls back to LOG_LEVEL.
func (h *AdminOps) ServeLogLevelUpdate(w http.ResponseWriter, r *http.Request) {
	var req logLevelRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}

	next, ok := parseLevelName(req.Level)
	if !ok {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "level must be one of debug, info, warn, error")
		return
	}

	from := levelName(h.logLevel.Level())
	h.logLevel.Set(next)
	to := levelName(next)

	actor, ip := h.actorAndIP(r)
	_ = h.store.RecordAdminAction(r.Context(), actor.Username, "admin.log_level.update", "log_level", "",
		detailJSON(fmt.Sprintf("log level changed from %s to %s", from, to),
			[]any{map[string]any{"field": "level", "from": from, "to": to}}), ip)

	writeAdminJSON(w, http.StatusOK, h.logLevelState())
}
