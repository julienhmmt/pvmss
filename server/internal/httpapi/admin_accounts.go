package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"pvmss/server/internal/logctx"
	"pvmss/server/internal/store"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	localUsernameMinLength = 3
	localUsernameMaxLength = 32
)

// localUsernamePattern keeps local account names distinguishable from PVE
// identities: lowercase alphanumerics, dot, underscore and hyphen, starting and
// ending with an alphanumeric. An "@" would read as a Proxmox realm.
var localUsernamePattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9._-]*[a-z0-9])?$`)

// AdminAccounts serves the owner-only management of DB-backed local
// administrators. Every route is additionally wrapped by the admin guard, so a
// non-admin never reaches the owner check.
type AdminAccounts struct {
	auth             *Auth
	store            *store.Store
	maxAccounts      int
	log              *slog.Logger
	trustedProxyHops int
}

// NewAdminAccounts builds the local account management handlers.
func NewAdminAccounts(authHandler *Auth, st *store.Store, maxAccounts int, log *slog.Logger) *AdminAccounts {
	return &AdminAccounts{auth: authHandler, store: st, maxAccounts: maxAccounts, log: log}
}

// SetTrustedProxyHops configures how many X-Forwarded-For hops are trusted
// when recording the client IP of an account mutation.
func (h *AdminAccounts) SetTrustedProxyHops(n int) {
	h.trustedProxyHops = n
}

type localAccountDTO struct {
	ID             int64  `json:"id"`
	Username       string `json:"username"`
	Disabled       bool   `json:"disabled"`
	FailedAttempts int    `json:"failedAttempts"`
	LockedUntil    string `json:"lockedUntil,omitempty"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
	CreatedBy      string `json:"createdBy"`
	LastLoginAt    string `json:"lastLoginAt,omitempty"`
}

type localAccountsResponse struct {
	Accounts []localAccountDTO `json:"accounts"`
	Max      int               `json:"max"`
	Count    int               `json:"count"`
}

type createLocalAccountRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type localAccountPasswordRequest struct {
	Password string `json:"password"`
}

type localAccountToggleRequest struct {
	Disabled bool `json:"disabled"`
}

// ServeList returns every local administrator plus the cap usage.
func (h *AdminAccounts) ServeList(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireOwner(w, r); !ok {
		return
	}

	accounts, err := h.store.LocalAccounts(r.Context())
	if err != nil {
		SetErrorMsg(w, "list local accounts failed", err)
		writeAdminError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	result := localAccountsResponse{Accounts: make([]localAccountDTO, 0, len(accounts)), Max: h.maxAccounts, Count: len(accounts)}
	for _, account := range accounts {
		result.Accounts = append(result.Accounts, toLocalAccountDTO(account))
	}

	writeAdminJSON(w, http.StatusOK, result)
}

// ServeCreate adds a local administrator. The cap excludes the bootstrap
// admin, so an instance with max=1 still has an owner plus one delegate.
func (h *AdminAccounts) ServeCreate(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.requireOwner(w, r)
	if !ok {
		return
	}

	var request createLocalAccountRequest
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

	count, err := h.store.CountLocalAccounts(r.Context())
	if err != nil {
		SetErrorMsg(w, "count local accounts failed", err)
		writeAdminError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	if count >= h.maxAccounts {
		writeAdminError(w, http.StatusConflict, "account_limit_reached", "local administrator limit reached")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(request.Password), bcrypt.DefaultCost)
	if err != nil {
		SetErrorMsg(w, "hash local account password failed", err)
		writeAdminError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	now := time.Now().UTC()
	account := store.LocalAccount{Username: username, PasswordHash: string(hash), CreatedAt: now, UpdatedAt: now, CreatedBy: actor.Username}

	id, err := h.store.CreateLocalAccount(r.Context(), account)
	if err != nil {
		if errors.Is(err, store.ErrDuplicate) {
			writeAdminError(w, http.StatusConflict, "username_taken", "a local account with that username already exists")
			return
		}

		SetErrorMsg(w, "create local account failed", err)
		writeAdminError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	created, err := h.store.LocalAccountByID(r.Context(), id)
	if err != nil {
		SetErrorMsg(w, "read created local account failed", err)
		writeAdminError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	h.recordAction(r, "admin.accounts.create", created.Username, "local admin account created", []any{
		auditChange("username", "", created.Username),
	})

	writeAdminJSON(w, http.StatusCreated, toLocalAccountDTO(created))
}

// ServeResetPassword replaces a password and clears any lockout. It revokes
// every session of the account, since the old secret may be compromised.
func (h *AdminAccounts) ServeResetPassword(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireOwner(w, r); !ok {
		return
	}

	id, ok := pathID(w, r)
	if !ok {
		return
	}

	var request localAccountPasswordRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "invalid password request")
		return
	}

	if err := validatePassword(request.Password); err != nil {
		writeAdminError(w, http.StatusBadRequest, "weak_password", err.Error())
		return
	}

	account, ok := h.accountByID(w, r, id)
	if !ok {
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(request.Password), bcrypt.DefaultCost)
	if err != nil {
		SetErrorMsg(w, "hash local account password failed", err)
		writeAdminError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	if err := h.store.SetLocalAccountPassword(r.Context(), id, string(hash), time.Now()); err != nil {
		h.writeAccountStoreFailure(w, "reset local account password failed", err)
		return
	}

	h.revokeSessions(r.Context(), account.Username)

	h.recordAction(r, "admin.accounts.password_reset", account.Username, "local admin password reset", []any{
		auditChange("password", "set", "reset"),
	})

	w.WriteHeader(http.StatusNoContent)
}

// ServeToggle enables or disables an account. Disabling revokes its sessions.
func (h *AdminAccounts) ServeToggle(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireOwner(w, r); !ok {
		return
	}

	id, ok := pathID(w, r)
	if !ok {
		return
	}

	var request localAccountToggleRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "invalid account request")
		return
	}

	account, ok := h.accountByID(w, r, id)
	if !ok {
		return
	}

	if err := h.store.SetLocalAccountDisabled(r.Context(), id, request.Disabled, time.Now()); err != nil {
		h.writeAccountStoreFailure(w, "toggle local account failed", err)
		return
	}

	if request.Disabled {
		h.revokeSessions(r.Context(), account.Username)
	}

	action := "admin.accounts.enable"
	summary := "local admin account enabled"

	if request.Disabled {
		action = "admin.accounts.disable"
		summary = "local admin account disabled"
	}

	h.recordAction(r, action, account.Username, summary, []any{auditChange("disabled", strconv.FormatBool(!request.Disabled), strconv.FormatBool(request.Disabled))})

	w.WriteHeader(http.StatusNoContent)
}

// ServeDelete removes an account and every session it owns. It is a hard
// delete: the username becomes reusable.
func (h *AdminAccounts) ServeDelete(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireOwner(w, r); !ok {
		return
	}

	id, ok := pathID(w, r)
	if !ok {
		return
	}

	account, ok := h.accountByID(w, r, id)
	if !ok {
		return
	}

	h.revokeSessions(r.Context(), account.Username)

	if err := h.store.DeleteLocalAccount(r.Context(), id); err != nil {
		h.writeAccountStoreFailure(w, "delete local account failed", err)
		return
	}

	h.recordAction(r, "admin.accounts.delete", account.Username, "local admin account deleted", []any{
		auditChange("username", account.Username, ""),
	})

	w.WriteHeader(http.StatusNoContent)
}

// requireOwner resolves the caller and enforces the owner role. It writes the
// rejection itself, so callers only branch on ok.
func (h *AdminAccounts) requireOwner(w http.ResponseWriter, r *http.Request) (actorIdentity, bool) {
	identity, err := h.auth.Principal(r)
	if err != nil {
		writeAuthError(w, http.StatusUnauthorized, "unauthenticated", msgAuthRequired)
		return actorIdentity{}, false
	}

	if !identity.Owner() {
		writeAdminError(w, http.StatusForbidden, "owner_only", msgOwnerOnly)
		return actorIdentity{}, false
	}

	return actorIdentity{Username: identity.Username}, true
}

type actorIdentity struct {
	Username string
}

func (h *AdminAccounts) accountByID(w http.ResponseWriter, r *http.Request, id int64) (store.LocalAccount, bool) {
	account, err := h.store.LocalAccountByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeAdminError(w, http.StatusNotFound, "not_found", "local account not found")
			return store.LocalAccount{}, false
		}

		SetErrorMsg(w, "read local account failed", err)
		writeAdminError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return store.LocalAccount{}, false
	}

	return account, true
}

func (h *AdminAccounts) writeAccountStoreFailure(w http.ResponseWriter, message string, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		writeAdminError(w, http.StatusNotFound, "not_found", "local account not found")
		return
	}

	SetErrorMsg(w, message, err)
	writeAdminError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)
}

func (h *AdminAccounts) revokeSessions(ctx context.Context, username string) {
	if err := h.store.DeleteSessionsForLocalUsername(ctx, username); err != nil {
		logctx.FromOr(ctx, h.log).WarnContext(ctx, "local account session revocation failed", "component", "httpapi", "error", err)
	}
}

func (h *AdminAccounts) recordAction(r *http.Request, action, targetID, summary string, changes []any) {
	actor, err := h.auth.Principal(r)
	if err != nil {
		return
	}

	if err := h.store.RecordAdminAction(r.Context(), actor.Username, action, "local_account", targetID, detailJSON(summary, changes), clientIP(r, h.trustedProxyHops)); err != nil {
		logctx.FromOr(r.Context(), h.log).WarnContext(r.Context(), "failed to record admin action", "component", "httpapi", "action", action, "error", err)
	}
}

func toLocalAccountDTO(account store.LocalAccount) localAccountDTO {
	dto := localAccountDTO{
		ID:             account.ID,
		Username:       account.Username,
		Disabled:       account.Disabled,
		FailedAttempts: account.FailedAttempts,
		CreatedAt:      account.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:      account.UpdatedAt.UTC().Format(time.RFC3339),
		CreatedBy:      account.CreatedBy,
	}

	if !account.LockedUntil.IsZero() {
		dto.LockedUntil = account.LockedUntil.UTC().Format(time.RFC3339)
	}

	if !account.LastLoginAt.IsZero() {
		dto.LastLoginAt = account.LastLoginAt.UTC().Format(time.RFC3339)
	}

	return dto
}

// validateLocalUsername normalises and checks a requested username.
func validateLocalUsername(raw string) (string, error) {
	username := strings.ToLower(strings.TrimSpace(raw))

	if username == bootstrapUsername {
		return "", errors.New(`"admin" is reserved for the bootstrap administrator`)
	}

	if len(username) < localUsernameMinLength || len(username) > localUsernameMaxLength {
		return "", errors.New("username must be 3 to 32 characters")
	}

	if !localUsernamePattern.MatchString(username) {
		return "", errors.New("username may only contain lowercase letters, digits, dot, underscore and hyphen, and must start and end with a letter or digit")
	}

	return username, nil
}

func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "invalid account id")
		return 0, false
	}

	return id, true
}

func auditChange(field, before, after string) map[string]string {
	return map[string]string{"field": field, "before": before, "after": after}
}
