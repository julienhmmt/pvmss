package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"pvmss/handlers"
)

// TestDocsDirectoryExists tests that the docs directory exists
func TestDocsDirectoryExists(t *testing.T) {
	// Check common locations for docs directory
	possiblePaths := []string{
		"../../docs",        // From backend/tests/ directory
		"../docs",           // From backend/ directory
		"./docs",            // From project root
		"/app/backend/docs", // Container path
	}

	found := false
	for _, path := range possiblePaths {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			found = true
			t.Logf("Found docs directory at: %s", path)
			break
		}
	}

	assert.True(t, found, "Docs directory should exist in one of the expected locations")
}

// TestDocsFilesExist tests that expected documentation files exist
func TestDocsFilesExist(t *testing.T) {
	// Find docs directory
	var docsDir string
	possiblePaths := []string{"../../docs", "../docs", "./docs", "/app/backend/docs"}

	for _, path := range possiblePaths {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			docsDir = path
			break
		}
	}

	if docsDir == "" {
		t.Skip("Docs directory not found, skipping file existence tests")
	}

	// Check for expected documentation files
	expectedFiles := []string{
		"user.en.md",
		"user.fr.md",
		"admin.en.md",
		"admin.fr.md",
	}

	for _, filename := range expectedFiles {
		filePath := filepath.Join(docsDir, filename)
		_, err := os.Stat(filePath)
		assert.NoError(t, err, "Documentation file %s should exist", filename)
	}
}

// TestNewDocsHandler tests that DocsHandler can be created
func TestNewDocsHandler(t *testing.T) {
	handler := handlers.NewDocsHandler()
	assert.NotNil(t, handler, "DocsHandler should be created successfully")
}
