package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"pvmss/server/internal/logctx"
	"pvmss/server/internal/store"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// setupAccountCreatedBy marks the local account created through the setup
// wizard. When no ADMIN_PASSWORD_HASH is configured it is the only identity
// allowed to manage local accounts (see AdminAccounts.requireOwner).
const setupAccountCreatedBy = "setup"

type setupAdminRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// setupStatusDTO is the GET /api/v1/setup response: which wizard steps the
// first-run page still has to complete.
type setupStatusDTO struct {
	// Required is true while no cluster exists - the application cannot serve
	// any VM surface until the first one is registered.
	Required bool `json:"required"`
	// NeedsAdmin is true while the instance has no administrator of its own:
	// no ADMIN_PASSWORD_HASH bootstrap and no local account. A Proxmox
	// administrator could still sign in, so NeedsAdmin alone does not set
	// Required.
	NeedsAdmin bool `json:"needsAdmin"`
	// NeedsCluster is true while the clusters table has no active row.
	NeedsCluster bool `json:"needsCluster"`
}

// Setup serves the pre-configuration status endpoint and the one-shot first
// administrator creation. It is public (pre-auth) by design: a fresh install
// has no session to authenticate with. Every write self-guards on the live
// database state, so the surface closes itself once setup completes.
type Setup struct {
	store        *store.Store
	hasBootstrap bool
	log          *slog.Logger
	// mu serializes needsAdmin+create so two concurrent first requests cannot
	// both slip an account through the one-shot window.
	mu sync.Mutex
}

// NewSetup builds the handler. hasBootstrap reports whether
// ADMIN_PASSWORD_HASH is configured - the env-seeded owner counts as an
// existing administrator for the NeedsAdmin check.
func NewSetup(st *store.Store, hasBootstrap bool, log *slog.Logger) *Setup {
	return &Setup{store: st, hasBootstrap: hasBootstrap, log: log}
}

// ServeStatus handles GET /api/v1/setup.
func (handler *Setup) ServeStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAdminError(w, http.StatusMethodNotAllowed, "method_not_allowed", msgMethodNotAllowed)
		return
	}
	status, err := handler.status(r)
	if err != nil {
		SetErrorMsg(w, "read setup status failed", err)
		writeAdminError(w, http.StatusInternalServerError, codeInternalError, msgInternalServerError)
		return
	}
	writeAdminJSON(w, http.StatusOK, status)
}

// needsAdmin reports whether the instance still lacks any administrator:
// no bootstrap owner from the environment and no local account row.
func (handler *Setup) needsAdmin(r *http.Request) (bool, error) {
	if handler.hasBootstrap {
		return false, nil
	}
	count, err := handler.store.CountLocalAccounts(r.Context())
	if err != nil {
		return false, err
	}
	return count == 0, nil
}

// ServeCreateAdmin handles POST /api/v1/setup/admin: one-shot creation of
// the first local administrator on an instance that has none. The endpoint
// is unauthenticated - it exists precisely because nobody can sign in yet -
// and closes itself as soon as any administrator exists (the needsAdmin
// check, re-evaluated on every call).
func (handler *Setup) ServeCreateAdmin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAdminError(w, http.StatusMethodNotAllowed, "method_not_allowed", msgMethodNotAllowed)
		return
	}
	handler.mu.Lock()
	defer handler.mu.Unlock()
	needsAdmin, err := handler.needsAdmin(r)
	if err != nil {
		SetErrorMsg(w, "read setup status failed", err)
		writeAdminError(w, http.StatusInternalServerError, codeInternalError, msgInternalServerError)
		return
	}
	if !needsAdmin {
		writeAdminError(w, http.StatusForbidden, "setup_complete", "an administrator already exists")
		return
	}
	var request setupAdminRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "invalid account request")
		return
	}
	username, err := validateLocalUsername(request.Username)
	if err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_username", err.Error())
		return
	}
	if err := validatePassword(request.Password); err != nil {
		writeAdminError(w, http.StatusBadRequest, "weak_password", err.Error())
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(request.Password), bcrypt.DefaultCost)
	if err != nil {
		SetErrorMsg(w, "hash setup account password failed", err)
		writeAdminError(w, http.StatusInternalServerError, codeInternalError, msgInternalServerError)
		return
	}
	now := time.Now().UTC()
	account := store.LocalAccount{Username: username, PasswordHash: string(hash), CreatedAt: now, UpdatedAt: now, CreatedBy: setupAccountCreatedBy}
	if _, err := handler.store.CreateLocalAccount(r.Context(), account); err != nil {
		if errors.Is(err, store.ErrDuplicate) {
			writeAdminError(w, http.StatusConflict, "username_taken", "a local account with that username already exists")
			return
		}
		SetErrorMsg(w, "create setup account failed", err)
		writeAdminError(w, http.StatusInternalServerError, codeInternalError, msgInternalServerError)
		return
	}
	if err := handler.store.RecordAdminAction(r.Context(), setupAccountCreatedBy, "setup.admin.create", "local_account", username, detailJSON("first administrator created through setup", nil), clientIP(r, 0)); err != nil {
		logctx.FromOr(r.Context(), handler.log).WarnContext(r.Context(), "failed to record setup admin action", "component", "httpapi", "error", err)
	}
	writeAdminJSON(w, http.StatusCreated, toLocalAccountDTO(account))
}

func (handler *Setup) status(r *http.Request) (setupStatusDTO, error) {
	rows, err := handler.store.ListClusters(r.Context())
	if err != nil {
		return setupStatusDTO{}, err
	}
	needsAdmin, err := handler.needsAdmin(r)
	if err != nil {
		return setupStatusDTO{}, err
	}
	needsCluster := len(rows) == 0
	return setupStatusDTO{Required: needsCluster, NeedsAdmin: needsAdmin, NeedsCluster: needsCluster}, nil
}
