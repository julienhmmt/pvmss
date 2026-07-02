package apiv1

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"

	"pvmss/state"
)

// decodeBody decodes the JSON request body into v. On failure it writes a 400
// bad-request response and returns false.
func decodeBody[T any](w http.ResponseWriter, r *http.Request, v *T) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		errBadRequest(w, "invalid JSON body")
		return false
	}
	return true
}

// requireVMID parses the `:id` route param as a positive integer. On failure
// writes a 400 response and returns (0, false).
func requireVMID(w http.ResponseWriter, r *http.Request) (int, bool) {
	ps := httprouter.ParamsFromContext(r.Context())
	vmid, err := strconv.Atoi(ps.ByName("id"))
	if err != nil || vmid <= 0 {
		errBadRequest(w, "invalid vm id")
		return 0, false
	}
	return vmid, true
}

// persistSettingsChange applies a settings mutation through the correct backend:
// when a DB is present it runs the DB-backed setter (which records the audit
// entry via changedBy); otherwise it applies the in-memory mutation and calls
// SetSettings. The in-memory path preserves the copyNodeLimits defensive copy so
// the stored settings never share the node-limits map with the live snapshot.
// Returns true on success; on failure it has already written the error response,
// so the caller must return immediately when it returns false.
func (h *AdminMutationsHandler) persistSettingsChange(
	w http.ResponseWriter,
	r *http.Request,
	dbWrite func(changedBy string) error,
	memMutate func(s *state.AppSettings) *state.AppSettings,
) bool {
	if h.state.HasDB() {
		if err := dbWrite(usernameFromCtx(r)); err != nil {
			writeAppError(w, err)
			return false
		}
		return true
	}
	settings := h.state.GetSettings()
	newSettings := memMutate(settings)
	newSettings.Limits.Nodes = copyNodeLimits(settings.Limits.Nodes)
	if err := h.state.SetSettings(newSettings); err != nil {
		writeAppError(w, err)
		return false
	}
	return true
}
