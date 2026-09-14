package vm

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/store"
)

// ConsolePasswordDeps bundles the collaborators SetConsolePassword needs.
// Reuses the same reader/writer/status-reader surface as the cloud-init
// config path so the agent-backed password apply is identical.
type ConsolePasswordDeps struct {
	Index        *inventory.Index
	Actor        auth.Identity
	ClusterName  string
	VMID         int
	Reader       cluster.CloudInitReader
	Writer       cluster.Writer
	Audit        *store.Store
	Refresher    IndexRefresher
	StatusReader cluster.VMStatusReader
}

// ErrConsolePasswordFailed is the sentinel for a console-password action that
// could not complete — the agent was unreachable, the VM was not running, or
// no cloud-init user is defined.
var ErrConsolePasswordFailed = errors.New("console password action failed")

// SetConsolePassword generates a random password server-side, applies it to
// the VM's own ciuser via the QEMU guest agent (never cipassword — the seed
// drive hash is readable by the tenant), and returns the password once for
// display. The password is not persisted, not logged, and not recorded in the
// audit trail.
func SetConsolePassword(ctx context.Context, deps ConsolePasswordDeps) (string, error) {
	entity, err := resolveCloudInitTarget(deps.Index, deps.Actor, deps.ClusterName, deps.VMID)
	if err != nil {
		return "", err
	}

	current, err := deps.Reader.GetCloudInitConfig(ctx, entity.Node, entity.VMID)
	if err != nil {
		return "", fmt.Errorf("%w: read cloud-init config: %w", ErrConsolePasswordFailed, err)
	}

	if current.User == "" {
		return "", ErrNoCloudInitUser
	}

	password, err := generateConsolePassword()
	if err != nil {
		return "", fmt.Errorf("%w: generate password: %w", ErrConsolePasswordFailed, err)
	}

	cloudInitDeps := CloudInitConfigDeps{
		Index: deps.Index, Actor: deps.Actor, ClusterName: deps.ClusterName, VMID: deps.VMID,
		Reader: deps.Reader, Writer: deps.Writer, Audit: deps.Audit, Refresher: deps.Refresher,
		StatusReader: deps.StatusReader,
	}

	if err := applyCloudInitPasswordFlow(ctx, cloudInitDeps, deps.Writer, entity, current, current, password); err != nil {
		return "", err
	}

	// Record the action without the password — the audit trail must not
	// carry the generated value.
	if err := deps.Audit.RecordAction(ctx, deps.Actor.Username, deps.ClusterName, deps.VMID, "set_console_password"); err != nil {
		return "", fmt.Errorf("record console-password audit: %w", err)
	}

	return password, nil
}

// generateConsolePassword returns a 16-byte random password, base64-encoded
// for console-safe characters. crypto/rand is the standard library's
// CSPRNG — math/rand is deliberately not used for a credential.
func generateConsolePassword() (string, error) {
	buf := make([]byte, 16)

	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(buf), nil
}
