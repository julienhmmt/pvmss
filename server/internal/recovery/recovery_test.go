// recovery_test.go is the CLI-level integration test plan.md names as this
// tranche's E2E-equivalent gate satisfier ("Playwright exercises
// browser-driven user journeys; this tranche has no browser-reachable
// surface... a shell-level integration test ... builds both binaries, runs
// them against fixtures, asserts exit codes and database contents"). It
// builds cmd/pvmss-recover and cmd/pvmss-checklist as real binaries and
// drives them as subprocesses, mirroring quickstart.md Steps 1 and 3.
package recovery_test

import (
	"bytes"
	"context"
	"database/sql"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// buildRecoveryBinary compiles a server/cmd/<name> binary into a temp dir
// and returns its path.
func buildRecoveryBinary(ctx context.Context, t *testing.T, repoRoot, cmdName string) string {
	t.Helper()

	bin := filepath.Join(t.TempDir(), cmdName)
	cmd := exec.CommandContext(ctx, "go", "build", "-o", bin, "./cmd/"+cmdName) //nolint:gosec // test builds a known repo-local command
	cmd.Dir = filepath.Join(repoRoot, "server")

	var stderr bytes.Buffer

	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("build %s: %v\n%s", cmdName, err, stderr.String())
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
	if ok := errorsAsExitError(err, &exitErr); ok {
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

// TestRecoveryCLI_EndToEnd runs pvmss-checklist and pvmss-recover as real
// subprocesses against file-backed fixtures, per quickstart.md Steps 1 and
// 3 and contracts/cutover.md's exit-code table.
// TestRecoveryCLI_EndToEnd builds cmd/pvmss-recover and cmd/pvmss-checklist as
// real binaries, drives them as subprocesses, and asserts the SC-004 golden
// summary plus the pvmss-recover happy-path and edge-case exit codes.
func TestRecoveryCLI_EndToEnd(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repoRoot := findRepoRootForCutover(t)

	recoverBin := buildRecoveryBinary(ctx, t, repoRoot, "pvmss-recover")
	checklistBin := buildRecoveryBinary(ctx, t, repoRoot, "pvmss-checklist")

	// --- pvmss-checklist: SUMMARY must match SC-004 exactly (quickstart Step 1) ---
	runChecklistGolden(ctx, t, checklistBin, repoRoot)

	// --- pvmss-recover: seed a legacy fixture, run against a migrated v0.4 db ---
	legacyPath := openAndSeedLegacyDB(ctx, t)
	v04Path := openAndMigrateV04DB(t)

	runRecoverGolden(ctx, t, recoverBin, legacyPath, v04Path)
	assertRecoverEdgeCases(ctx, t, recoverBin, legacyPath, v04Path)
}

// openAndSeedLegacyDB creates a file-backed legacy fixture database, applies the
// legacy schema, seeds it with the default dataset, and returns its path.
// Extracted from TestRecoveryCLI_EndToEnd to keep that test's Cognitive
// Complexity under the go:S3776 threshold.
func openAndSeedLegacyDB(ctx context.Context, t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "legacy.db")

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open legacy fixture: %v", err)
	}

	if _, err := db.ExecContext(ctx, legacySchemaDDL); err != nil {
		t.Fatalf("create legacy schema: %v", err)
	}

	seedLegacyDB(t, db, defaultSeed())

	if err := db.Close(); err != nil {
		t.Fatalf("close legacy fixture: %v", err)
	}

	return path
}

// openAndMigrateV04DB creates a file-backed v0.4 fixture database, runs the
// v0.4 migrations, and returns its path.
func openAndMigrateV04DB(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "v04.db")

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open v0.4 fixture: %v", err)
	}

	if err := storeRunMigrations(db); err != nil {
		t.Fatalf("migrate v0.4 fixture: %v", err)
	}

	if err := db.Close(); err != nil {
		t.Fatalf("close v0.4 fixture: %v", err)
	}

	return path
}

// runChecklistGolden runs pvmss-checklist and asserts the SC-004 golden
// summary (quickstart.md Step 1): "58 fiches found" and the exact SUMMARY
// line "53 closed, 5 open (3 real gaps, 2 deliberate design decisions)".
func runChecklistGolden(ctx context.Context, t *testing.T, bin, repoRoot string) {
	t.Helper()

	checklistOut, err := exec.CommandContext(ctx, bin, "--repo-root", repoRoot).CombinedOutput() //nolint:gosec // test invokes its own freshly built binary
	if err != nil {
		t.Fatalf("pvmss-checklist: %v\n%s", err, checklistOut)
	}

	if !strings.Contains(string(checklistOut), "58 fiches found") {
		t.Errorf("pvmss-checklist output missing fiche count:\n%s", checklistOut)
	}

	if !strings.Contains(string(checklistOut), "SUMMARY: 53 closed, 5 open (3 real gaps, 2 deliberate design decisions)") {
		t.Errorf("pvmss-checklist SUMMARY does not match SC-004:\n%s", checklistOut)
	}
}

// runRecoverCmd runs pvmss-recover against the given fixture paths and
// returns its combined output and exit code.
func runRecoverCmd(ctx context.Context, t *testing.T, bin, legacyPath, v04Path string) (string, int) {
	t.Helper()

	cmd := exec.CommandContext(ctx, bin, //nolint:gosec // test invokes its own freshly built binary
		"--legacy-db", legacyPath,
		"--v0.4-db", v04Path,
		"--cluster-name", "test-cluster",
		"--session-secret", "test-session-secret-at-least-32-bytes!!",
	)

	out, err := cmd.CombinedOutput()

	return string(out), exitCodeOf(t, err)
}

// runRecoverGolden runs pvmss-recover once and asserts the happy path
// (quickstart.md Step 3): exit 0, a SUMMARY line, and the recovered
// database contents match defaultSeed()'s known values.
func runRecoverGolden(ctx context.Context, t *testing.T, bin, legacyPath, v04Path string) {
	t.Helper()

	out, code := runRecoverCmd(ctx, t, bin, legacyPath, v04Path)
	if code != 0 {
		t.Fatalf("pvmss-recover exit=%d, want 0:\n%s", code, out)
	}

	if !strings.Contains(out, "SUMMARY: written=") {
		t.Errorf("pvmss-recover output missing SUMMARY line:\n%s", out)
	}

	// Inspect the target database directly (quickstart.md Step 3 validate block).
	assertRecoveredDB(ctx, t, v04Path)
}

// assertRecoveredDB checks the v0.4 database directly against
// defaultSeed()'s known values, per quickstart.md Step 3's validate block
// and SC-002 (the three no-legacy-source vm_limits fields must be T12's
// shipped defaults, never anything read from the legacy row).
func assertRecoveredDB(ctx context.Context, t *testing.T, v04Path string) {
	t.Helper()

	db, err := sql.Open("sqlite", v04Path)
	if err != nil {
		t.Fatalf("reopen v0.4 db: %v", err)
	}
	defer func() { _ = db.Close() }()

	if n := countRows(t, db, `SELECT COUNT(*) FROM clusters WHERE name = ?`, "test-cluster"); n != 1 {
		t.Errorf("clusters count = %d, want 1", n)
	}

	if n := countRows(t, db, `SELECT COUNT(*) FROM catalog_nodes WHERE cluster = ?`, "test-cluster"); n != 2 {
		t.Errorf("catalog_nodes count = %d, want 2", n)
	}

	var (
		sockets, cores, memMB, diskGB, netCards, snapshots, vmPerUser int
		allowYAML                                                     bool
	)

	err = db.QueryRowContext(ctx, `
		SELECT max_sockets, max_cores, max_memory_mb, max_disk_per_vm_gb,
		       max_network_cards, max_snapshots, max_vm_per_user, allow_custom_yaml
		FROM vm_limits WHERE cluster = ?`, "test-cluster").Scan(
		&sockets, &cores, &memMB, &diskGB, &netCards, &snapshots, &vmPerUser, &allowYAML)
	if err != nil {
		t.Fatalf("query vm_limits: %v", err)
	}

	if sockets != 4 || cores != 8 || memMB != 16384 {
		t.Errorf("vm_limits no-source fields = (%d,%d,%d), want T12 defaults (4,8,16384)", sockets, cores, memMB)
	}

	if diskGB != 20 || netCards != 3 || snapshots != 8 || vmPerUser != 5 || !allowYAML {
		t.Errorf("vm_limits copied fields = (%d,%d,%d,%d,%v), want (20,3,8,5,true)",
			diskGB, netCards, snapshots, vmPerUser, allowYAML)
	}
}

// assertRecoverEdgeCases exercises the pvmss-recover CLI contract beyond the
// happy path: SC-003 idempotence (a second run produces identical row
// counts, no duplicates) and the contracts/cutover.md exit-code table
// (invalid cluster name -> 2, unreadable legacy db -> 1).
func assertRecoverEdgeCases(ctx context.Context, t *testing.T, bin, legacyPath, v04Path string) {
	t.Helper()

	// --- Idempotence (SC-003): re-running produces identical row counts ---
	if _, code := runRecoverCmd(ctx, t, bin, legacyPath, v04Path); code != 0 {
		t.Fatalf("second pvmss-recover run exit=%d, want 0", code)
	}

	inspectDB2, err := sql.Open("sqlite", v04Path)
	if err != nil {
		t.Fatalf("reopen v0.4 db after second run: %v", err)
	}
	defer func() { _ = inspectDB2.Close() }()

	if n := countRows(t, inspectDB2, `SELECT COUNT(*) FROM catalog_nodes WHERE cluster = ?`, "test-cluster"); n != 2 {
		t.Errorf("catalog_nodes count after second run = %d, want 2 (no duplicates)", n)
	}

	// --- Exit code 2: invalid cluster name (contracts/cutover.md) ---
	badNameCmd := exec.CommandContext(ctx, bin, //nolint:gosec // test invokes its own freshly built binary
		"--legacy-db", legacyPath,
		"--v0.4-db", v04Path,
		"--cluster-name", "Not Valid!",
		"--session-secret", "test-session-secret-at-least-32-bytes!!",
	)
	if code := exitCodeOf(t, badNameCmd.Run()); code != 2 {
		t.Errorf("pvmss-recover with invalid --cluster-name: exit=%d, want 2", code)
	}

	// --- Exit code 1: unreadable --legacy-db ---
	badPathCmd := exec.CommandContext(ctx, bin, //nolint:gosec // test invokes its own freshly built binary
		"--legacy-db", filepath.Join(t.TempDir(), "does-not-exist.db"),
		"--v0.4-db", v04Path,
		"--cluster-name", "test-cluster",
	)
	if code := exitCodeOf(t, badPathCmd.Run()); code != 1 {
		t.Errorf("pvmss-recover with missing --legacy-db: exit=%d, want 1", code)
	}
}
