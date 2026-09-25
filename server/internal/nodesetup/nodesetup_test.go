package nodesetup_test

import (
	"bytes"
	"os"
	"path/filepath"
	"pvmss/server/internal/nodesetup"
	"testing"
)

// TestScript_MatchesToolsCopy fails when the embedded script and the copy
// published in tools/ drift apart.
func TestScript_MatchesToolsCopy(t *testing.T) {
	t.Parallel()

	want, err := os.ReadFile(filepath.Join("..", "..", "..", "tools", "pvmss-node-setup.sh"))
	if err != nil {
		t.Fatalf("read tools/pvmss-node-setup.sh: %v", err)
	}

	if !bytes.Equal(nodesetup.Script(), want) {
		t.Fatal("server/internal/nodesetup/pvmss-node-setup.sh and tools/pvmss-node-setup.sh differ: " +
			"edit tools/pvmss-node-setup.sh, then copy it to server/internal/nodesetup/")
	}
}

// TestScript_IsShellScript guards against an empty or truncated embed.
func TestScript_IsShellScript(t *testing.T) {
	t.Parallel()

	got := nodesetup.Script()
	if !bytes.HasPrefix(got, []byte("#!/bin/sh\n")) {
		t.Fatalf("script does not start with a sh shebang: %q", got[:min(len(got), 20)])
	}

	got[0] = 'X'
	if nodesetup.Script()[0] != '#' {
		t.Fatal("Script returned the embedded bytes instead of a copy")
	}
}
