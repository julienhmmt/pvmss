package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// findRepoRoot walks up from the test's working directory to locate the
// pvmss repository root by looking for server/cmd.
func findRepoRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	for range 10 {
		if _, err := os.Stat(filepath.Join(dir, "server", "cmd")); err == nil {
			return dir
		}

		dir = filepath.Dir(dir)
	}

	t.Fatal("could not find repo root containing server/cmd")

	return ""
}

// buildChecklistBinary compiles cmd/pvmss-checklist into a temp dir and
// returns its path.
func buildChecklistBinary(ctx context.Context, t *testing.T, repoRoot string) string {
	t.Helper()

	bin := filepath.Join(t.TempDir(), "pvmss-checklist")
	cmd := exec.CommandContext(ctx, "go", "build", "-o", bin, "./cmd/pvmss-checklist") //nolint:gosec // test builds a known repo-local command
	cmd.Dir = filepath.Join(repoRoot, "server")

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("build pvmss-checklist: %v\n%s", err, stderr.String())
	}

	return bin
}

// exitCodeOf extracts the process exit code from exec.Command.Run's error,
// failing the test if the command could not even start.
func exitCodeOf(t *testing.T, err error) int {
	t.Helper()

	if err == nil {
		return 0
	}

	var exitErr *exec.ExitError
	if errorsAsExitError(err, &exitErr) {
		return exitErr.ExitCode()
	}

	t.Fatalf("command did not start: %v", err)

	return -1
}

func errorsAsExitError(err error, target **exec.ExitError) bool {
	exitErr, ok := err.(*exec.ExitError) //nolint:errorlint // exec.Command errors are never wrapped
	if !ok {
		return false
	}

	*target = exitErr

	return true
}

func TestChecklistCLI_HappyPath(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repoRoot := findRepoRoot(t)
	bin := buildChecklistBinary(ctx, t, repoRoot)

	out, err := exec.CommandContext(ctx, bin, "--repo-root", repoRoot).CombinedOutput() //nolint:gosec // test invokes its own freshly built binary
	if err != nil {
		t.Fatalf("pvmss-checklist: %v\n%s", err, out)
	}

	output := string(out)
	if !strings.Contains(output, "fiches found") {
		t.Errorf("output missing fiche count:\n%s", output)
	}

	if !strings.Contains(output, "SUMMARY:") {
		t.Errorf("output missing SUMMARY line:\n%s", output)
	}
}

func TestChecklistCLI_ExitCodeZeroOnSuccess(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repoRoot := findRepoRoot(t)
	bin := buildChecklistBinary(ctx, t, repoRoot)

	out, err := exec.CommandContext(ctx, bin, "--repo-root", repoRoot).CombinedOutput() //nolint:gosec // test invokes its own freshly built binary
	if code := exitCodeOf(t, err); code != 0 {
		t.Fatalf("pvmss-checklist exit=%d, want 0:\n%s", code, out)
	}
}

func TestChecklistCLI_MissingFicheDirs_ExitZero(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repoRoot := findRepoRoot(t)
	bin := buildChecklistBinary(ctx, t, repoRoot)

	emptyRoot := t.TempDir()
	out, err := exec.CommandContext(ctx, bin, "--repo-root", emptyRoot).CombinedOutput() //nolint:gosec // test invokes its own freshly built binary
	if code := exitCodeOf(t, err); code != 0 {
		t.Fatalf("pvmss-checklist with empty repo-root exit=%d, want 0:\n%s", code, out)
	}

	if !strings.Contains(string(out), "0 fiches found") {
		t.Errorf("expected 0 fiches for empty repo, got:\n%s", out)
	}
}

func TestChecklistCLI_UnreadableDir_ExitOne(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repoRoot := findRepoRoot(t)
	bin := buildChecklistBinary(ctx, t, repoRoot)

	unreadable := filepath.Join(t.TempDir(), "unreadable")
	if err := os.MkdirAll(filepath.Join(unreadable, ".claude", "v0.4", "auth"), 0o750); err != nil {
		t.Fatalf("create unreadable fiche dir: %v", err)
	}

	if err := os.Chmod(filepath.Join(unreadable, ".claude", "v0.4", "auth"), 0o000); err != nil {
		t.Fatalf("chmod unreadable: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(unreadable, ".claude", "v0.4", "auth"), 0o750) })

	if code := exitCodeOf(t, exec.CommandContext(ctx, bin, "--repo-root", unreadable).Run()); code != 1 { //nolint:gosec // test invokes its own freshly built binary
		t.Fatalf("pvmss-checklist with unreadable dir exit=%d, want 1", code)
	}
}

func TestChecklistCLI_DefaultRepoRoot(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repoRoot := findRepoRoot(t)
	bin := buildChecklistBinary(ctx, t, repoRoot)

	cmd := exec.CommandContext(ctx, bin) //nolint:gosec // test invokes its own freshly built binary
	cmd.Dir = repoRoot

	out, err := cmd.CombinedOutput()
	if code := exitCodeOf(t, err); code != 0 {
		t.Fatalf("pvmss-checklist with default repo-root exit=%d, want 0:\n%s", code, out)
	}

	if !strings.Contains(string(out), "fiches found") {
		t.Errorf("output missing fiche count with default repo-root:\n%s", out)
	}
}
