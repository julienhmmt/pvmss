//nolint:noctx // test scaffolding does not need real context
package httpapi_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"pvmss/server/internal/httpapi"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

const (
	profileSSHKeysPath   = "/api/v1/profile/ssh-keys"
	profileSSHKeysUser   = `{"username":"alice","password":"pvmss-alice"}`
	profileSSHKeysUserB  = `{"username":"bob","password":"pvmss-bob"}`
	profileSSHKeysMaxLen = 1024
)

func newTestSSHPublicKey(t *testing.T, comment string) string {
	t.Helper()

	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate ed25519 key: %v", err)
	}

	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatalf("convert public key: %v", err)
	}

	key := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(sshPub)))
	if comment != "" {
		key += " " + comment
	}

	return key
}

func newProfileSSHKeysRouter(t *testing.T, logger *slog.Logger) (http.Handler, *httpapi.Auth) {
	t.Helper()

	authHandler, st := newAuthHandlerWithStore(t)
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(testWriter{t}, nil))
	}

	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	return httpapi.NewRouter(httpapi.RouterConfig{
		Health:         ok,
		ClusterNodes:   ok,
		ClusterRefresh: ok,
		VMs:            ok,
		VMDetail:       ok,
		Auth:           authHandler,
		ProfileSSHKeys: httpapi.NewProfileSSHKeys(authHandler, st, logger),
		Store:          st,
		Log:            logger,
	}), authHandler
}

func serveProfileSSHKeys(t *testing.T, mux http.Handler, method, path, body string, session, csrf *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()

	var reader *strings.Reader
	if body != "" {
		reader = strings.NewReader(body)
	} else {
		reader = strings.NewReader("")
	}

	req := httptest.NewRequest(method, path, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	if session != nil {
		req.AddCookie(session)
	}

	if csrf != nil {
		req.AddCookie(csrf)
		req.Header.Set("X-CSRF-Token", csrf.Value)
	}

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	return rec
}

type profileSSHKeyResponse struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	PublicKey   string `json:"publicKey"`
	Fingerprint string `json:"fingerprint"`
	CreatedAt   string `json:"createdAt"`
}

type profileSSHKeyListBody struct {
	Keys []profileSSHKeyResponse `json:"keys"`
}

type profileSSHKeyErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func createProfileSSHKey(t *testing.T, mux http.Handler, session, csrf *http.Cookie, label, publicKey string) *httptest.ResponseRecorder {
	t.Helper()

	body, err := json.Marshal(map[string]string{"label": label, "publicKey": publicKey})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	return serveProfileSSHKeys(t, mux, http.MethodPost, profileSSHKeysPath, string(body), session, csrf)
}

func decodeProfileSSHKeyError(t *testing.T, rec *httptest.ResponseRecorder) profileSSHKeyErrorBody {
	t.Helper()

	var body profileSSHKeyErrorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body %q: %v", rec.Body.String(), err)
	}

	return body
}

//nolint:paralleltest // serial: shared database fixture
func TestProfileSSHKeys_Unauthenticated(t *testing.T) {
	mux, _ := newProfileSSHKeysRouter(t, nil)

	cases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "list", method: http.MethodGet, path: profileSSHKeysPath},
		{name: "create", method: http.MethodPost, path: profileSSHKeysPath, body: `{"label":"x","publicKey":"ssh-ed25519 AAAA"}`},
		{name: "delete", method: http.MethodDelete, path: profileSSHKeysPath + "/some-id"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := serveProfileSSHKeys(t, mux, tc.method, tc.path, tc.body, nil, nil)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("%s %s: status = %d, want 401", tc.method, tc.path, rec.Code)
			}
		})
	}
}

//nolint:paralleltest // serial: shared database fixture
func TestProfileSSHKeys_WriteRequiresCSRF(t *testing.T) {
	mux, authHandler := newProfileSSHKeysRouter(t, nil)
	session, _ := loginCSRF(t, authHandler, profileSSHKeysUser)

	key := newTestSSHPublicKey(t, "")

	rec := createProfileSSHKey(t, mux, session, nil, "laptop", key)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("POST without csrf: status = %d, want 403", rec.Code)
	}

	rec = serveProfileSSHKeys(t, mux, http.MethodDelete, profileSSHKeysPath+"/some-id", "", session, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("DELETE without csrf: status = %d, want 403", rec.Code)
	}
}

func postAndDecodeKey(t *testing.T, mux http.Handler, session, csrf *http.Cookie, label, publicKey string) profileSSHKeyResponse {
	t.Helper()

	rec := createProfileSSHKey(t, mux, session, csrf, label, publicKey)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201: %s", rec.Code, rec.Body.String())
	}

	var created profileSSHKeyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create body: %v", err)
	}

	return created
}

func listProfileSSHKeys(t *testing.T, mux http.Handler, session *http.Cookie) []profileSSHKeyResponse {
	t.Helper()

	rec := serveProfileSSHKeys(t, mux, http.MethodGet, profileSSHKeysPath, "", session, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200", rec.Code)
	}

	var list profileSSHKeyListBody
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list body: %v", err)
	}

	return list.Keys
}

//nolint:paralleltest // serial: shared database fixture
func TestProfileSSHKeys_CreateAndList(t *testing.T) {
	mux, authHandler := newProfileSSHKeysRouter(t, nil)
	session, csrf := loginCSRF(t, authHandler, profileSSHKeysUser)

	publicKey := newTestSSHPublicKey(t, "alice@laptop")
	created := postAndDecodeKey(t, mux, session, csrf, "laptop", publicKey)

	if created.ID == "" || created.Label != "laptop" || created.PublicKey != publicKey {
		t.Fatalf("created = %+v, want id set, label laptop and the posted key", created)
	}

	if !strings.HasPrefix(created.Fingerprint, "SHA256:") || created.CreatedAt == "" {
		t.Fatalf("created = %+v, want SHA256 fingerprint and createdAt", created)
	}

	keys := listProfileSSHKeys(t, mux, session)
	if len(keys) != 1 || keys[0].ID != created.ID {
		t.Fatalf("list = %+v, want the created key", keys)
	}
}

//nolint:paralleltest // serial: shared database fixture
func TestProfileSSHKeys_Delete(t *testing.T) {
	mux, authHandler := newProfileSSHKeysRouter(t, nil)
	session, csrf := loginCSRF(t, authHandler, profileSSHKeysUser)

	created := postAndDecodeKey(t, mux, session, csrf, "laptop", newTestSSHPublicKey(t, "alice@laptop"))

	rec := serveProfileSSHKeys(t, mux, http.MethodDelete, profileSSHKeysPath+"/"+created.ID, "", session, csrf)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204", rec.Code)
	}

	if keys := listProfileSSHKeys(t, mux, session); len(keys) != 0 {
		t.Fatalf("list after delete = %+v, want empty", keys)
	}
}

//nolint:paralleltest // serial: shared database fixture
func TestProfileSSHKeys_CreateValidation(t *testing.T) {
	mux, authHandler := newProfileSSHKeysRouter(t, nil)
	session, csrf := loginCSRF(t, authHandler, profileSSHKeysUser)

	validKey := newTestSSHPublicKey(t, "")
	otherKey := newTestSSHPublicKey(t, "")

	cases := []struct {
		name       string
		body       string
		wantStatus int
		wantCode   string
	}{
		{name: "malformed json", body: `{"label":`, wantStatus: http.StatusBadRequest, wantCode: "invalid_request"},
		{name: "unknown field", body: `{"label":"x","publicKey":"` + validKey + `","extra":1}`, wantStatus: http.StatusBadRequest, wantCode: "invalid_request"},
		{name: "empty label", body: `{"label":"  ","publicKey":"` + validKey + `"}`, wantStatus: http.StatusBadRequest, wantCode: "invalid_label"},
		{name: "label too long", body: `{"label":"` + strings.Repeat("l", 65) + `","publicKey":"` + validKey + `"}`, wantStatus: http.StatusBadRequest, wantCode: "invalid_label"},
		{name: "empty key", body: `{"label":"x","publicKey":" "}`, wantStatus: http.StatusBadRequest, wantCode: "ssh_key_empty"},
		{name: "multiline key", body: `{"label":"x","publicKey":"ssh-ed25519 AAAA\nssh-ed25519 BBBB"}`, wantStatus: http.StatusBadRequest, wantCode: "ssh_key_multiline"},
		{name: "bad type", body: `{"label":"x","publicKey":"ssh-bad AAAA"}`, wantStatus: http.StatusBadRequest, wantCode: "ssh_key_type"},
		{name: "unparseable blob", body: `{"label":"x","publicKey":"ssh-ed25519 AAAA"}`, wantStatus: http.StatusBadRequest, wantCode: "ssh_key_format"},
		{name: "private key", body: `{"label":"x","publicKey":"-----BEGIN OPENSSH PRIVATE KEY-----\nabc\n-----END OPENSSH PRIVATE KEY-----"}`, wantStatus: http.StatusBadRequest, wantCode: "ssh_key_private"},
		{name: "key too long", body: `{"label":"x","publicKey":"` + validKey + " " + strings.Repeat("x", profileSSHKeysMaxLen) + `"}`, wantStatus: http.StatusBadRequest, wantCode: "ssh_key_too_long"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertCreateKeyRejected(t, mux, session, csrf, tc.body, tc.wantStatus, tc.wantCode)
		})
	}

	// A stored key then rejects both a re-used label and a re-used blob.
	if rec := createProfileSSHKey(t, mux, session, csrf, "laptop", validKey); rec.Code != http.StatusCreated {
		t.Fatalf("seed create status = %d: %s", rec.Code, rec.Body.String())
	}

	t.Run("duplicate label", func(t *testing.T) {
		assertCreateKeyConflict(t, mux, session, csrf, "laptop", otherKey, "duplicate_label")
	})

	t.Run("duplicate key with different comment", func(t *testing.T) {
		repasted := strings.Fields(validKey)[0] + " " + strings.Fields(validKey)[1] + " pasted-again"
		assertCreateKeyConflict(t, mux, session, csrf, "copy", repasted, "duplicate_key")
	})
}

// assertCreateKeyRejected POSTs body to the create endpoint and wants
// wantStatus with error code wantCode.
func assertCreateKeyRejected(t *testing.T, mux http.Handler, session, csrf *http.Cookie, body string, wantStatus int, wantCode string) {
	t.Helper()

	rec := serveProfileSSHKeys(t, mux, http.MethodPost, profileSSHKeysPath, body, session, csrf)
	if rec.Code != wantStatus {
		t.Fatalf("status = %d, want %d: %s", rec.Code, wantStatus, rec.Body.String())
	}

	if errBody := decodeProfileSSHKeyError(t, rec); errBody.Code != wantCode {
		t.Fatalf("code = %q, want %q", errBody.Code, wantCode)
	}
}

// assertCreateKeyConflict creates a key and wants a 409 with error code
// wantCode.
func assertCreateKeyConflict(t *testing.T, mux http.Handler, session, csrf *http.Cookie, label, publicKey, wantCode string) {
	t.Helper()

	rec := createProfileSSHKey(t, mux, session, csrf, label, publicKey)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rec.Code)
	}

	if errBody := decodeProfileSSHKeyError(t, rec); errBody.Code != wantCode {
		t.Fatalf("code = %q, want %q", errBody.Code, wantCode)
	}
}

//nolint:paralleltest // serial: shared database fixture
func TestProfileSSHKeys_Limit(t *testing.T) {
	mux, authHandler := newProfileSSHKeysRouter(t, nil)
	session, csrf := loginCSRF(t, authHandler, profileSSHKeysUser)

	for i := range 10 {
		rec := createProfileSSHKey(t, mux, session, csrf, fmt.Sprintf("key-%d", i), newTestSSHPublicKey(t, ""))
		if rec.Code != http.StatusCreated {
			t.Fatalf("create %d status = %d: %s", i, rec.Code, rec.Body.String())
		}
	}

	rec := createProfileSSHKey(t, mux, session, csrf, "eleventh", newTestSSHPublicKey(t, ""))
	if rec.Code != http.StatusConflict {
		t.Fatalf("eleventh key status = %d, want 409", rec.Code)
	}

	if body := decodeProfileSSHKeyError(t, rec); body.Code != "limit_reached" {
		t.Fatalf("code = %q, want limit_reached", body.Code)
	}
}

//nolint:paralleltest // serial: shared database fixture
func TestProfileSSHKeys_DeleteScoped(t *testing.T) {
	mux, authHandler := newProfileSSHKeysRouter(t, nil)
	aliceSession, aliceCSRF := loginCSRF(t, authHandler, profileSSHKeysUser)
	bobSession, bobCSRF := loginCSRF(t, authHandler, profileSSHKeysUserB)

	rec := createProfileSSHKey(t, mux, aliceSession, aliceCSRF, "laptop", newTestSSHPublicKey(t, ""))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d: %s", rec.Code, rec.Body.String())
	}

	var created profileSSHKeyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create body: %v", err)
	}

	// Bob cannot delete Alice's key id, and a random id is not found either.
	rec = serveProfileSSHKeys(t, mux, http.MethodDelete, profileSSHKeysPath+"/"+created.ID, "", bobSession, bobCSRF)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("cross-user delete status = %d, want 404", rec.Code)
	}

	if body := decodeProfileSSHKeyError(t, rec); body.Code != "not_found" {
		t.Fatalf("code = %q, want not_found", body.Code)
	}

	rec = serveProfileSSHKeys(t, mux, http.MethodDelete, profileSSHKeysPath+"/does-not-exist", "", aliceSession, aliceCSRF)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown id delete status = %d, want 404", rec.Code)
	}
}

//nolint:paralleltest // serial: shared database fixture
func TestProfileSSHKeys_LogsFingerprintNotKeyBody(t *testing.T) {
	var buf bytes.Buffer

	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	mux, authHandler := newProfileSSHKeysRouter(t, logger)
	session, csrf := loginCSRF(t, authHandler, profileSSHKeysUser)

	publicKey := newTestSSHPublicKey(t, "alice@laptop")
	blob := strings.Fields(publicKey)[1]

	rec := createProfileSSHKey(t, mux, session, csrf, "laptop", publicKey)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d: %s", rec.Code, rec.Body.String())
	}

	var created profileSSHKeyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create body: %v", err)
	}

	rec = serveProfileSSHKeys(t, mux, http.MethodDelete, profileSSHKeysPath+"/"+created.ID, "", session, csrf)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204", rec.Code)
	}

	output := buf.String()
	if !strings.Contains(output, created.Fingerprint) {
		t.Fatalf("log output %q does not contain fingerprint %q", output, created.Fingerprint)
	}

	if !strings.Contains(output, "laptop") {
		t.Fatalf("log output %q does not contain the label", output)
	}

	if strings.Contains(output, blob) {
		t.Fatalf("log output leaks the key blob: %q", output)
	}
}
