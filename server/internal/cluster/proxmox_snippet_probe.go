package cluster

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ErrSnippetTargetMismatch reports that the configured snippet directory is
// not what Proxmox serves as <storage>:snippets/: a file written there is not
// listed by the storage. It wraps ErrSnippetWriteUnavailable so every caller
// that already treats "no write target" as "feature off" does the same here
// - the one outcome that must never happen is a cicustom pointing at a file
// Proxmox cannot see, which makes every later start fail with
// "volume '<storage>:snippets/<file>' does not exist".
var ErrSnippetTargetMismatch = fmt.Errorf("%w: the snippet directory is not the storage's snippets/ directory", ErrSnippetWriteUnavailable)

// snippetProbeTTL bounds how long a successful probe is trusted. A mount can
// disappear under a running process, so the proof is renewed periodically
// instead of once at startup; failures are never cached, so fixing the mount
// takes effect on the next create.
const snippetProbeTTL = 5 * time.Minute

var snippetProbeCache = struct {
	sync.Mutex
	ok map[string]time.Time
}{ok: make(map[string]time.Time)}

func (p Proxmox) snippetProbeKey(node string) string {
	return fmt.Sprintf("%s|%s|%s|%s|%t", p.BaseURL, p.SnippetDir, p.SnippetStorage, node, p.SSH.Enabled())
}

// verifySnippetTarget proves end to end that a file PVMSS writes into its
// snippet directory is visible to Proxmox as <SnippetStorage>:snippets/<file>
// on node: write a throwaway probe file, list the storage's snippets content
// through the API, remove the probe. It runs BEFORE any VM is created, so a
// misconfigured mount (the classic case: a bind mount of a local directory
// instead of the storage's snippets/ dir) is refused up front instead of
// producing a VM whose start fails.
func (p Proxmox) verifySnippetTarget(ctx context.Context, node string) error {
	key := p.snippetProbeKey(node)

	snippetProbeCache.Lock()
	at, cached := snippetProbeCache.ok[key]
	snippetProbeCache.Unlock()

	if cached && time.Since(at) < snippetProbeTTL {
		return nil
	}

	suffix := make([]byte, 6)
	if _, err := rand.Read(suffix); err != nil {
		return fmt.Errorf("snippet probe name: %w", err)
	}

	filename := "pvmss-probe-" + hex.EncodeToString(suffix) + ".yml"

	if err := p.PushCloudInitSnippet(ctx, node, p.SnippetStorage, filename, 0, "#cloud-config\n# PVMSS write-target probe - safe to delete\n"); err != nil {
		return fmt.Errorf("%w: write probe into %q: %w", ErrSnippetTargetMismatch, p.SnippetDir, err)
	}

	visible, listErr := p.HasSnippet(ctx, node, p.SnippetStorage, filename)

	// Best-effort: a leftover probe file is cosmetic, never a failure.
	_ = p.removeProbe(ctx, node, filename)

	if listErr != nil {
		return fmt.Errorf("list %s snippets on %s: %w", p.SnippetStorage, node, listErr)
	}

	if !visible {
		return fmt.Errorf("%w: a file written to %q is not listed as %s:snippets/ on node %s - mount that storage's snippets/ directory there, or enable SSH delivery", ErrSnippetTargetMismatch, p.SnippetDir, p.SnippetStorage, node)
	}

	snippetProbeCache.Lock()
	snippetProbeCache.ok[key] = time.Now()
	snippetProbeCache.Unlock()

	return nil
}

// removeProbe deletes the probe on the same node it was written to (the
// generic RemoveCloudInitSnippet targets the API host over SSH).
func (p Proxmox) removeProbe(ctx context.Context, node, filename string) error {
	if p.SSH.Enabled() {
		return p.sshRemoveSnippet(ctx, node, filename)
	}

	if err := os.Remove(filepath.Join(p.SnippetDir, filename)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	return nil
}
