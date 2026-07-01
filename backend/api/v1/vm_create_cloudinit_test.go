package apiv1

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"pvmss/proxmox"
	"pvmss/state"
)

// uploadFixture wires a VMCreateHandler with injectable SFTP and API upload
// functions and tracks which path was attempted.
type uploadFixture struct {
	h            *VMCreateHandler
	sftpCalled   bool
	apiCalled    bool
	sftpErr      error
	apiErr       error
	sftpEnabled  bool
	snippetStore string
}

func newUploadFixture(t *testing.T, sftpEnabled bool) *uploadFixture {
	t.Helper()
	f := &uploadFixture{sftpEnabled: sftpEnabled, snippetStore: "local"}
	f.h = &VMCreateHandler{
		state: state.MakeAppState(),
		uploadSnippetSFTP: func(_ context.Context, _ proxmox.CloudInitSFTPConfig, _, _ string) error {
			f.sftpCalled = true
			return f.sftpErr
		},
		uploadSnippetAPI: func(_ context.Context, _ *proxmox.RestyClient, _, _, _, _ string) error {
			f.apiCalled = true
			return f.apiErr
		},
	}
	return f
}

func (f *uploadFixture) settings() *state.AppSettings {
	return &state.AppSettings{
		CloudInitSFTP: proxmox.CloudInitSFTPConfig{Enabled: f.sftpEnabled},
	}
}

func TestUploadSnippet_SFTPDisabled_APISuccess(t *testing.T) {
	f := newUploadFixture(t, false)
	f.apiErr = nil

	cicustom, warn := f.h.uploadSnippet(context.Background(), nil, "node1", f.snippetStore, "pvmss-100.yml", "#cloud-config", f.settings())

	assert.Equal(t, "user=local:snippets/pvmss-100.yml", cicustom)
	assert.Empty(t, warn)
	assert.False(t, f.sftpCalled, "SFTP should not be tried when disabled")
	assert.True(t, f.apiCalled, "API upload should be attempted")
}

func TestUploadSnippet_SFTPDisabled_APIFailure(t *testing.T) {
	f := newUploadFixture(t, false)
	f.apiErr = errors.New("400 bad request")

	cicustom, warn := f.h.uploadSnippet(context.Background(), nil, "node1", f.snippetStore, "pvmss-100.yml", "#cloud-config", f.settings())

	// This is the user's reported scenario: SFTP off + Proxmox API returns 400.
	assert.Empty(t, cicustom, "no cicustom on failure")
	assert.Equal(t, "upload-failed-api", warn, "must surface the API-specific warning so users learn SFTP is required")
	assert.False(t, f.sftpCalled)
	assert.True(t, f.apiCalled)
}

func TestUploadSnippet_SFTPEnabled_SFTPSuccess(t *testing.T) {
	f := newUploadFixture(t, true)
	f.sftpErr = nil

	cicustom, warn := f.h.uploadSnippet(context.Background(), nil, "node1", f.snippetStore, "pvmss-100.yml", "#cloud-config", f.settings())

	assert.Equal(t, "user=local:snippets/pvmss-100.yml", cicustom)
	assert.Empty(t, warn)
	assert.True(t, f.sftpCalled)
	assert.False(t, f.apiCalled, "API fallback should not run when SFTP succeeds")
}

func TestUploadSnippet_SFTPEnabled_SFTPFailure_APIFallbackSuccess(t *testing.T) {
	f := newUploadFixture(t, true)
	f.sftpErr = errors.New("ssh: handshake failed")
	f.apiErr = nil

	cicustom, warn := f.h.uploadSnippet(context.Background(), nil, "node1", f.snippetStore, "pvmss-100.yml", "#cloud-config", f.settings())

	assert.Equal(t, "user=local:snippets/pvmss-100.yml", cicustom)
	assert.Empty(t, warn, "API fallback success means no warning")
	assert.True(t, f.sftpCalled)
	assert.True(t, f.apiCalled, "API fallback should run when SFTP fails")
}

func TestUploadSnippet_SFTPEnabled_BothFail(t *testing.T) {
	f := newUploadFixture(t, true)
	f.sftpErr = errors.New("ssh: handshake failed")
	f.apiErr = errors.New("400 bad request")

	cicustom, warn := f.h.uploadSnippet(context.Background(), nil, "node1", f.snippetStore, "pvmss-100.yml", "#cloud-config", f.settings())

	assert.Empty(t, cicustom)
	assert.Equal(t, "upload-failed-sftp", warn, "when SFTP was enabled but both failed, surface the SFTP-specific warning")
	assert.True(t, f.sftpCalled)
	assert.True(t, f.apiCalled)
}
