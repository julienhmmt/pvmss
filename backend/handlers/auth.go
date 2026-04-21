package handlers

import (
	"net/http"

	"github.com/julienschmidt/httprouter"

	"pvmss/state"
)

// AuthHandler handles authentication routes
type AuthHandler struct {
	stateManager state.StateManager
}

// MakeAuthHandler creates a new instance of AuthHandler
func MakeAuthHandler(sm state.StateManager) *AuthHandler {
	return &AuthHandler{stateManager: sm}
}

// RegisterRoutes registers authentication routes.
// All login/logout flows are handled by the SvelteKit SPA and the api/v1 JWT stack.
// No legacy session routes remain.
func (h *AuthHandler) RegisterRoutes(_ *httprouter.Router) {}

// RedirectIfAuthenticated is middleware that redirects authenticated users away from login page.
func (h *AuthHandler) RedirectIfAuthenticated(next httprouter.Handle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		if IsAuthenticated(r) {
			http.Redirect(w, r, "/vm/create", http.StatusSeeOther)
			return
		}
		next(w, r, ps)
	}
}
