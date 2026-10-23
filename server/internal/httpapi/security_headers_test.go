package httpapi

import (
	"crypto/sha256"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSPASecurityPolicy_AllowsOnlyShellInlineScriptsByHash: the SPA CSP used
// 'unsafe-inline', so any injected inline script would run. It now allows
// exactly the inline scripts of the built shell, by hash.
func TestSPASecurityPolicy_AllowsOnlyShellInlineScriptsByHash(t *testing.T) {
	t.Parallel()

	theme := "\n\t\t\ttry { document.documentElement.classList.add('dark'); } catch {}\n\t\t"
	start := "\n\t\t\t{ __sveltekit_x = {}; }\n\t\t"
	dir := t.TempDir()
	index := "<html><head><script>" + theme + "</script><script type=\"module\" src=\"/_app/a.js\"></script></head>" +
		"<body><script>" + start + "</script></body></html>"

	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte(index), 0o600); err != nil {
		t.Fatalf("write index: %v", err)
	}

	policy := spaSecurityPolicy(dir)

	for _, script := range []string{theme, start} {
		sum := sha256.Sum256([]byte(script))
		if hash := "'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"; !strings.Contains(policy, hash) {
			t.Errorf("policy %q lacks hash %s", policy, hash)
		}
	}

	if strings.Contains(policy, "script-src 'self' 'unsafe-inline'") {
		t.Errorf("policy still allows any inline script: %q", policy)
	}

	if strings.Count(policy, "'sha256-") != 2 {
		t.Errorf("policy %q, want exactly two script hashes (the src script is not hashed)", policy)
	}

	if strings.Contains(policy, "https:") {
		t.Errorf("policy %q still allows remote images", policy)
	}

	if got := spaSecurityPolicy(""); !strings.Contains(got, "script-src 'self';") {
		t.Errorf("policy without a shell = %q, want script-src 'self' only", got)
	}
}
