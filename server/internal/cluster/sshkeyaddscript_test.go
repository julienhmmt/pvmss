//nolint:wsl_v5 // script-execution tests keep setup, stub, and assertions together
package cluster

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// writeGetentStub installs a fake getent in binDir that resolves a user only
// when a directory named after them exists under homes. getent is invoked as
// `getent passwd <user>`, so the user is $2. It prints a passwd line whose
// owner fields are the test process's own uid/gid — chown to self is
// permitted without root, so the script's real chown line is exercised — and
// whose home field points at the per-user directory.
func writeGetentStub(t *testing.T, binDir, homes string) {
	t.Helper()

	stub := "#!/bin/sh\n" +
		"user=$2\n" +
		"if [ -d \"" + homes + "/$user\" ]; then\n" +
		"	echo \"$user:x:" + itoa(os.Getuid()) + ":" + itoa(os.Getgid()) + "::" + homes + "/$user:/bin/sh\"\n" +
		"	exit 0\n" +
		"fi\n" +
		"exit 2\n"

	if err := os.WriteFile(filepath.Join(binDir, "getent"), []byte(stub), 0o755); err != nil { //nolint:gosec // test-only stub in a temp dir, deliberately executable
		t.Fatalf("write getent stub: %v", err)
	}
}

// itoa converts a small int for the stub's passwd line.
func itoa(n int) string {
	return strconv.Itoa(n)
}

// runSSHKeyAddScript executes sshKeyAddScript through /bin/sh with the stub
// PATH, returning the guest's exit status and stderr. The user and key travel
// as argv, exactly as the guest agent passes them.
func runSSHKeyAddScript(t *testing.T, binDir, user, key string) (int, string) {
	t.Helper()

	scriptPath := filepath.Join(t.TempDir(), "add-key.sh")
	if err := os.WriteFile(scriptPath, []byte(sshKeyAddScript), 0o755); err != nil { //nolint:gosec // test-only script in a temp dir, deliberately executable
		t.Fatalf("write script: %v", err)
	}

	stderr := &bytes.Buffer{}

	// Drop the inherited PATH before prepending binDir: getenv resolves the
	// FIRST match, so a duplicated entry would leave the real PATH winning
	// and the getent stub unreachable.
	env := make([]string, 0, len(os.Environ())+1)
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "PATH=") {
			env = append(env, entry)
		}
	}

	cmd := exec.CommandContext(t.Context(), "/bin/sh", scriptPath, user, key) //nolint:gosec // test runs the fixed script constant in a temp dir
	env = append(env, "PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	cmd.Env = env
	cmd.Stderr = stderr

	err := cmd.Run()

	exit := 0
	var exitErr *exec.ExitError
	switch {
	case errors.As(err, &exitErr):
		exit = exitErr.ExitCode()
	case err != nil:
		t.Fatalf("run script: %v", err)
	}

	return exit, stderr.String()
}

// TestSSHKeyAddScript_Behaviour exercises the guest-side script as a real
// /bin/sh program (ticket 07): the append must be newline-safe and
// idempotent, and a missing user must exit 3.
func TestSSHKeyAddScript_Behaviour(t *testing.T) {
	t.Parallel()

	key := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5 alice@laptop"

	tests := []struct {
		name      string
		existing  string
		addTwice  bool
		userFound bool
		wantLines int
		wantExit  int
	}{
		{name: "absent file is created with the key", userFound: true, wantLines: 1, wantExit: 0},
		{name: "no trailing newline does not glue keys", existing: "ssh-ed25519 AAAA-old bob@laptop", userFound: true, wantLines: 2, wantExit: 0},
		{name: "duplicate add is a no-op", existing: key + "\n", addTwice: true, userFound: true, wantLines: 1, wantExit: 0},
		{name: "two different keys make two lines", existing: "ssh-ed25519 AAAA-old bob@laptop\n", addTwice: true, userFound: true, wantLines: 2, wantExit: 0},
		{name: "empty existing file does not abort under set -e", addTwice: true, userFound: true, wantLines: 1, wantExit: 0},
		{name: "unknown user exits 3", userFound: false, wantLines: 0, wantExit: 3},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			binDir, homes := t.TempDir(), t.TempDir()
			writeGetentStub(t, binDir, homes)

			home := filepath.Join(homes, "debian")
			if tc.userFound {
				if err := os.MkdirAll(home, 0o750); err != nil {
					t.Fatalf("mkdir home: %v", err)
				}
			}

			auth := filepath.Join(home, ".ssh", "authorized_keys")
			if tc.existing != "" {
				if err := os.MkdirAll(filepath.Dir(auth), 0o750); err != nil {
					t.Fatalf("mkdir .ssh: %v", err)
				}

				if err := os.WriteFile(auth, []byte(tc.existing), 0o600); err != nil {
					t.Fatalf("seed authorized_keys: %v", err)
				}
			}

			exit, stderr := runSSHKeyAddScript(t, binDir, "debian", key)
			if exit != tc.wantExit {
				t.Fatalf("exit = %d (stderr %q), want %d", exit, stderr, tc.wantExit)
			}

			if tc.wantExit != 0 {
				return
			}

			raw, err := os.ReadFile(auth) //nolint:gosec // test-only path built from t.TempDir
			if err != nil {
				t.Fatalf("read authorized_keys: %v", err)
			}

			lines := nonEmptyLines(string(raw))
			if len(lines) != tc.wantLines {
				t.Fatalf("authorized_keys = %q (%d lines), want %d lines", string(raw), len(lines), tc.wantLines)
			}

			if !strings.HasSuffix(string(raw), "\n") {
				t.Errorf("authorized_keys does not end with a newline: %q", string(raw))
			}
		})
	}
}

// TestSSHKeyAddScript_Permissions verifies the file and directory modes the
// script enforces (600 / 700).
func TestSSHKeyAddScript_Permissions(t *testing.T) {
	t.Parallel()

	binDir, homes := t.TempDir(), t.TempDir()
	writeGetentStub(t, binDir, homes)

	home := filepath.Join(homes, "debian")
	if err := os.MkdirAll(home, 0o750); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}

	if exit, stderr := runSSHKeyAddScript(t, binDir, "debian", "ssh-ed25519 AAAA x"); exit != 0 {
		t.Fatalf("exit = %d (stderr %q), want 0", exit, stderr)
	}

	auth := filepath.Join(home, ".ssh", "authorized_keys")

	authInfo, err := os.Stat(auth)
	if err != nil {
		t.Fatalf("stat authorized_keys: %v", err)
	}

	if authInfo.Mode().Perm() != 0o600 {
		t.Errorf("authorized_keys mode = %o, want 600", authInfo.Mode().Perm())
	}

	sshInfo, err := os.Stat(filepath.Join(home, ".ssh"))
	if err != nil {
		t.Fatalf("stat .ssh: %v", err)
	}

	if sshInfo.Mode().Perm() != 0o700 {
		t.Errorf(".ssh mode = %o, want 700", sshInfo.Mode().Perm())
	}
}

// nonEmptyLines splits s into non-empty lines.
func nonEmptyLines(s string) []string {
	var out []string

	for line := range strings.SplitSeq(s, "\n") {
		if line != "" {
			out = append(out, line)
		}
	}

	return out
}
