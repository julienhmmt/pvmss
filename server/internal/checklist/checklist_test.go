package checklist_test

import (
	"bytes"
	"pvmss/server/internal/checklist"
	"pvmss/server/internal/testfixture"
	"strings"
	"testing"
)

// Walking.claude/v0.4/{auth,vm,admin,plateforme}/ against a fixture
// directory tree returns exactly the fiche count expected.
func TestGenerate_FicheCount(t *testing.T) {
	t.Parallel()

	// Generate the fiche tree so the test does not depend on gitignored files.
	repoRoot := testfixture.ChecklistFiches(t)

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

// Every fiche in the "none" list is reported as
// NONE, and every other fiche is reported with a non-empty tranche label.
// The SUMMARY line must match "53 closed, 5 open (3 real gaps, 2 deliberate)".
func TestGenerate_NoneFichesReportedCorrectly(t *testing.T) {
	t.Parallel()

	repoRoot := testfixture.ChecklistFiches(t)

	var buf bytes.Buffer
	if err := checklist.Generate(&buf, repoRoot); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	output := buf.String()

	// The five still-open "NONE" rows, each tagged deliberate or gap.
	// The rest were closed and are no longer NONE.
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

	// SUMMARY line must match exactly
	if !strings.Contains(output, "SUMMARY: 53 closed, 5 open (3 real gaps, 2 deliberate design decisions)") {
		t.Errorf("output missing or incorrect SUMMARY line:\n%s", output)
	}
}

// Deliberate vs gap distinction - are "deliberate", not "gap".
func TestGenerate_DeliberateVsGap(t *testing.T) {
	t.Parallel()

	repoRoot := testfixture.ChecklistFiches(t)

	var buf bytes.Buffer
	if err := checklist.Generate(&buf, repoRoot); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	output := buf.String()

	// Should say "deliberate"
	for _, fiche := range []string{"X12", "X18"} {
		if line := findFicheLine(output, fiche); line != "" {
			if !strings.Contains(line, "deliberate") {
				t.Errorf("%s should be 'deliberate': %s", fiche, line)
			}
		}
	}

	// The remaining real gaps.
	for _, fiche := range []string{"X13", "P01", "P02"} {
		line := findFicheLine(output, fiche)
		if !strings.Contains(line, "real gap") {
			t.Errorf("fiche %s should say 'real gap'", fiche)
		}
	}
}

// findFicheLine returns the first line of output that starts with fiche+" ",
// or "" if no such line exists.
func findFicheLine(output, fiche string) string {
	for line := range strings.SplitSeq(output, "\n") {
		if strings.HasPrefix(line, fiche+"  ") {
			return line
		}
	}

	return ""
}

// Fixture directory tree with missing subdirectories still works.
func TestGenerate_MissingFicheDir(t *testing.T) {
	t.Parallel()

	//  Use a temp dir with no.claude/v0.4/ - should produce 0 fiches
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
