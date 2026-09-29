package httpapi

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cloudinit"
	"pvmss/server/internal/store"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	// profileSSHKeyMaxKeys caps how many SSH public keys one
	// (cluster, username) profile can store.
	profileSSHKeyMaxKeys = 10
	// profileSSHKeyIDBytes is the entropy size of a generated key id, hex
	// encoded into the id itself.
	profileSSHKeyIDBytes = 16
	// profileSSHKeyLabelMaxRunes is the inclusive rune cap for a key label.
	profileSSHKeyLabelMaxRunes = 64
)

// ProfileSSHKeys serves GET/POST/DELETE /api/v1/profile/ssh-keys, the SSH
// public keys a user saves for reuse in cloud-init forms. Every route is
// scoped to the session identity's (cluster, username) pair.
type ProfileSSHKeys struct {
	auth *Auth
	st   *store.Store
	log  *slog.Logger
}

// NewProfileSSHKeys builds the profile SSH key handler.
func NewProfileSSHKeys(authHandler *Auth, st *store.Store, log *slog.Logger) *ProfileSSHKeys {
	return &ProfileSSHKeys{auth: authHandler, st: st, log: log}
}

type profileSSHKeyDTO struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	PublicKey   string `json:"publicKey"`
	Fingerprint string `json:"fingerprint"`
	CreatedAt   string `json:"createdAt"`
}

type profileSSHKeyCreateRequest struct {
	Label     string `json:"label"`
	PublicKey string `json:"publicKey"`
}

// ServeHTTP dispatches by method; the mux only routes GET and POST on the
// collection path and DELETE on the {id} path to this handler.
func (h *ProfileSSHKeys) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	identity, err := h.auth.Principal(r)
	if err != nil {
		writeAuthError(w, http.StatusUnauthorized, "unauthenticated", msgAuthRequired)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.listKeys(w, r, identity)
	case http.MethodPost:
		h.createKey(w, r, identity)
	case http.MethodDelete:
		h.deleteKey(w, r, identity)
	default:
		w.Header().Set("Allow", "GET, POST, DELETE")
		writeAuthError(w, http.StatusMethodNotAllowed, "method_not_allowed", msgMethodNotAllowed)
	}
}

func (h *ProfileSSHKeys) listKeys(w http.ResponseWriter, r *http.Request, identity auth.Identity) {
	keys, err := h.st.ListProfileSSHKeys(r.Context(), identity.Cluster, identity.Username)
	if err != nil {
		h.writeInternal(w, err)
		return
	}

	dtos := make([]profileSSHKeyDTO, 0, len(keys))
	for _, key := range keys {
		dto, err := profileSSHKeyToDTO(key)
		if err != nil {
			h.writeInternal(w, err)
			return
		}

		dtos = append(dtos, dto)
	}

	h.writeJSON(w, http.StatusOK, struct {
		Keys []profileSSHKeyDTO `json:"keys"`
	}{Keys: dtos})
}

func (h *ProfileSSHKeys) createKey(w http.ResponseWriter, r *http.Request, identity auth.Identity) {
	var request profileSSHKeyCreateRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeAuthError(w, http.StatusBadRequest, "invalid_request", msgInvalidRequestBody)
		return
	}

	label := strings.TrimSpace(request.Label)
	publicKey := strings.TrimSpace(request.PublicKey)

	if n := utf8.RuneCountInString(label); n < 1 || n > profileSSHKeyLabelMaxRunes {
		writeAuthError(w, http.StatusBadRequest, "invalid_label", "label must be between 1 and 64 characters")
		return
	}

	fingerprint, err := cloudinit.ValidateProfileSSHKey(publicKey)
	if err != nil {
		writeAuthError(w, http.StatusBadRequest, sshKeyErrorCode(err), err.Error())
		return
	}

	key := store.ProfileSSHKey{Cluster: identity.Cluster, Username: identity.Username, Label: label, PublicKey: publicKey, CreatedAt: time.Now().UTC()}
	if key.ID, err = newProfileSSHKeyID(); err != nil {
		h.writeInternal(w, err)
		return
	}

	if err := h.st.CreateProfileSSHKey(r.Context(), key, profileSSHKeyMaxKeys); err != nil {
		h.writeStoreError(w, err)
		return
	}

	h.log.Info("profile ssh key added", "component", "httpapi", "user", identity.Username, "cluster", identity.Cluster, "fingerprint", fingerprint, "label", label)

	dto, err := profileSSHKeyToDTO(key)
	if err != nil {
		h.writeInternal(w, err)
		return
	}

	h.writeJSON(w, http.StatusCreated, dto)
}

func (h *ProfileSSHKeys) deleteKey(w http.ResponseWriter, r *http.Request, identity auth.Identity) {
	id := r.PathValue("id")

	keys, err := h.st.ListProfileSSHKeys(r.Context(), identity.Cluster, identity.Username)
	if err != nil {
		h.writeInternal(w, err)
		return
	}

	var target *store.ProfileSSHKey

	for i := range keys {
		if keys[i].ID == id {
			target = &keys[i]
			break
		}
	}

	if target == nil {
		writeAuthError(w, http.StatusNotFound, "not_found", "ssh key not found")
		return
	}

	if err := h.st.DeleteProfileSSHKey(r.Context(), identity.Cluster, identity.Username, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeAuthError(w, http.StatusNotFound, "not_found", "ssh key not found")
			return
		}

		h.writeInternal(w, err)

		return
	}

	fingerprint, err := cloudinit.SSHKeyFingerprint(target.PublicKey)
	if err != nil {
		fingerprint = "unknown"
	}

	h.log.Info("profile ssh key deleted", "component", "httpapi", "user", identity.Username, "cluster", identity.Cluster, "fingerprint", fingerprint, "label", target.Label)

	w.WriteHeader(http.StatusNoContent)
}

// writeStoreError maps the store sentinels to their public conflict codes.
func (h *ProfileSSHKeys) writeStoreError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrProfileSSHKeyDuplicateLabel):
		writeAuthError(w, http.StatusConflict, "duplicate_label", "a key with this label already exists")
	case errors.Is(err, store.ErrProfileSSHKeyDuplicate):
		writeAuthError(w, http.StatusConflict, "duplicate_key", "this ssh key is already saved")
	case errors.Is(err, store.ErrProfileSSHKeyLimit):
		writeAuthError(w, http.StatusConflict, "limit_reached", "the maximum number of ssh keys is reached")
	default:
		h.writeInternal(w, err)
	}
}

func (h *ProfileSSHKeys) writeInternal(w http.ResponseWriter, err error) {
	h.log.Error("profile ssh key request failed", "component", "httpapi", "error", err)
	writeAuthError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)
}

func (h *ProfileSSHKeys) writeJSON(w http.ResponseWriter, status int, value any) {
	body, err := json.Marshal(value)
	if err != nil {
		h.log.Error("failed to marshal profile ssh key response", "component", "httpapi", "error", err)
		writeAuthError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	if err := writeJSON(w, status, body); err != nil {
		h.log.Error("failed to write profile ssh key response", "component", "httpapi", "error", err)
	}
}

// sshKeyErrorCode maps a cloudinit validation failure to its stable code.
func sshKeyErrorCode(err error) string {
	switch {
	case errors.Is(err, cloudinit.ErrSSHKeyEmpty):
		return "ssh_key_empty"
	case errors.Is(err, cloudinit.ErrSSHKeyMultiline):
		return "ssh_key_multiline"
	case errors.Is(err, cloudinit.ErrSSHKeyType):
		return "ssh_key_type"
	case errors.Is(err, cloudinit.ErrSSHKeyPrivate):
		return "ssh_key_private"
	case errors.Is(err, cloudinit.ErrSSHKeyTooLong):
		return "ssh_key_too_long"
	default:
		return "ssh_key_format"
	}
}

func profileSSHKeyToDTO(key store.ProfileSSHKey) (profileSSHKeyDTO, error) {
	fingerprint, err := cloudinit.SSHKeyFingerprint(key.PublicKey)
	if err != nil {
		return profileSSHKeyDTO{}, fmt.Errorf("fingerprint stored ssh key: %w", err)
	}

	return profileSSHKeyDTO{
		ID:          key.ID,
		Label:       key.Label,
		PublicKey:   key.PublicKey,
		Fingerprint: fingerprint,
		CreatedAt:   key.CreatedAt.Format(time.RFC3339),
	}, nil
}

func newProfileSSHKeyID() (string, error) {
	buf := make([]byte, profileSSHKeyIDBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("read ssh key id entropy: %w", err)
	}

	return hex.EncodeToString(buf), nil
}
