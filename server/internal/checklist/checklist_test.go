package checklist_test

import (
	"bytes"
	"os"
	"path/filepath"
	"pvmss/server/internal/checklist"
	"strings"
	"testing"
)

// T023: walking .claude/v0.4/{auth,vm,admin,plateforme}/ against a fixture
// directory tree returns exactly the fiche count expected.
func TestGenerate_FicheCount(t *testing.T) {
	t.Parallel()

	// Use the real repo root — the fiche directories are part of the repo.
	repoRoot := findRepoRoot(t)

	var buf bytes.Buffer
	if err := checklist.Generate(&buf, repoRoot); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "58 fiches found") {
		t.Errorf("output does not contain '58 fiches found':\n%s", output)
	}

	if !strings.Contains(output, "auth=6") {
		t.Errorf("output does not contain 'auth=6':\n%s", output)
	}

	if !strings.Contains(output, "vm=27") {
		t.Errorf("output does not contain 'vm=27':\n%s", output)
	}

	if !strings.Contains(output, "admin=19") {
		t.Errorf("output does not contain 'admin=19':\n%s", output)
	}

	if !strings.Contains(output, "plateforme=6") {
		t.Errorf("output does not contain 'plateforme=6':\n%s", output)
	}
}

// T024 / SC-004: every fiche in spec.md's FR-006 "none" list is reported as
// NONE, and every other fiche is reported with a non-empty tranche label.
// The SUMMARY line must match "53 closed, 5 open (3 real gaps, 2 deliberate)".
func TestGenerate_NoneFichesReportedCorrectly(t *testing.T) {
	t.Parallel()

	repoRoot := findRepoRoot(t)

	var buf bytes.Buffer
	if err := checklist.Generate(&buf, repoRoot); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	output := buf.String()

	// The five still-open "NONE" rows: X12 (deliberate), X13 (gap), X18 (deliberate), P01 (gap), P02 (gap).
	// V13, X11, P03-P06 were closed (→ T17, T18, T19) and are no longer NONE.
	noneFiches := []string{"X12", "X13", "X18", "P01", "P02"}
	for _, fiche := range noneFiches {
		if !strings.Contains(output, fiche+"  ") {
			t.Errorf("output missing fiche %s", fiche)
		}
		// Each NONE fiche should have "→ NONE" in its line
		for line := range strings.SplitSeq(output, "\n") {
			if strings.HasPrefix(line, fiche+"  ") {
				if !strings.Contains(line, "NONE") {
					t.Errorf("fiche %s line does not say NONE: %s", fiche, line)
				}
			}
		}
	}

	// SC-004: SUMMARY line must match exactly
	if !strings.Contains(output, "SUMMARY: 53 closed, 5 open (3 real gaps, 2 deliberate design decisions)") {
		t.Errorf("output missing or incorrect SUMMARY line:\n%s", output)
	}
}

// T024: deliberate vs gap distinction — X12 and X18 are "deliberate", not "gap".
func TestGenerate_DeliberateVsGap(t *testing.T) {
	t.Parallel()

	repoRoot := findRepoRoot(t)

	var buf bytes.Buffer
	if err := checklist.Generate(&buf, repoRoot); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	output := buf.String()

	// X12 and X18 should say "deliberate"
	for _, fiche := range []string{"X12", "X18"} {
		if line := findFicheLine(output, fiche); line != "" {
			if !strings.Contains(line, "deliberate") {
				t.Errorf("%s should be 'deliberate': %s", fiche, line)
			}
		}
	}

	// X13, P01, P02 remain real gaps (V13, X11, P03-P06 closed via T17/T18/T19).
	for _, fiche := range []string{"X13", "P01", "P02"} {
		line := findFicheLine(output, fiche)
		if !strings.Contains(line, "real gap") {
			t.Errorf("fiche %s should say 'real gap'", fiche)
		}
	}
}

// findFicheLine returns the first line of output that starts with fiche+"  ",
// or "" if no such line exists.
func findFicheLine(output, fiche string) string {
	for line := range strings.SplitSeq(output, "\n") {
		if strings.HasPrefix(line, fiche+"  ") {
			return line
		}
	}

	return ""
}

// T023: fixture directory tree with missing subdirectories still works.
func TestGenerate_MissingFicheDir(t *testing.T) {
	t.Parallel()

	// Use a temp dir with no .claude/v0.4/ — should produce 0 fiches
	repoRoot := t.TempDir()

	var buf bytes.Buffer
	if err := checklist.Generate(&buf, repoRoot); err != nil {
		t.Fatalf("Generate with missing dirs: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "0 fiches found") {
		t.Errorf("expected 0 fiches for empty repo, got:\n%s", output)
	}
}

// findRepoRoot returns the repository root by looking for the .claude directory.
func findRepoRoot(t *testing.T) string {
	t.Helper()
	// Walk up from the current directory to find .claude/v0.4/auth/
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	for range 10 {
		candidate := filepath.Join(dir, ".claude", "v0.4", "auth")
		if fi, err := os.Stat(candidate); err == nil && fi.IsDir() {
			return dir
		}

		dir = filepath.Dir(dir)
	}

	t.Fatal("could not find repo root with .claude/v0.4/auth/")

	return ""
}
