package httpapi

import (
	"net/http"
	"pvmss/server/internal/nodesetup"
	"strconv"
)

// nodeSetupScriptPath serves tools/pvmss-node-setup.sh, embedded in the
// binary, so a Proxmox node can run
//
//	curl -fsSL https://<pvmss>/api/v1/pvmss-node-setup.sh | sh -s -- --storage ... --user ... --key '...'
//
// The script holds no secret (the key passed to it is PVMSS's public key), so
// the route is public: a node has no PVMSS session.
const nodeSetupScriptPath = "/api/v1/pvmss-node-setup.sh"

// serveNodeSetupScript writes the script as plain text, so a browser shows
// it (Infrastructure > Clusters links to it) instead of downloading it.
func serveNodeSetupScript(w http.ResponseWriter, r *http.Request) {
	script := nodesetup.Script()

	h := w.Header()
	h.Set("Content-Type", "text/plain; charset=utf-8")
	h.Set("Content-Disposition", `inline; filename="pvmss-node-setup.sh"`)
	h.Set("Content-Length", strconv.Itoa(len(script)))
	w.WriteHeader(http.StatusOK)

	if r.Method == http.MethodHead {
		return
	}

	_, _ = w.Write(script)
}
