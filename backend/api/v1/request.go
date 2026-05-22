package apiv1

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
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
