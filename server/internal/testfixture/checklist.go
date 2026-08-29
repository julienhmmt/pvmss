// Package testfixture provides hermetic test fixtures shared across packages.
package testfixture

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

type ficheDir struct {
	dir    string
	prefix string
	count  int
}

var ficheDirs = []ficheDir{
	{"auth", "A", 6},
	{"vm", "V", 27},
	{"admin", "X", 19},
	{"plateforme", "P", 6},
}

// ChecklistFiches creates a temporary repository containing all 58 checklist fiches.
func ChecklistFiches(tb testing.TB) string {
	tb.Helper()

	repoRoot := tb.TempDir()

	for _, fixture := range ficheDirs {
		writeFiches(tb, repoRoot, fixture)
	}

	return repoRoot
}

func writeFiches(tb testing.TB, repoRoot string, fixture ficheDir) {
	tb.Helper()

	dirPath := filepath.Join(repoRoot, ".claude", "v0.4", fixture.dir)

	if err := os.MkdirAll(dirPath, 0o750); err != nil {
		tb.Fatalf("create fixture directory: %v", err)
	}

	for i := 1; i <= fixture.count; i++ {
		filename := fmt.Sprintf("%s%02d-fixture.md", fixture.prefix, i)

		if err := os.WriteFile(filepath.Join(dirPath, filename), nil, 0o600); err != nil {
			tb.Fatalf("write fixture file: %v", err)
		}
	}
}
