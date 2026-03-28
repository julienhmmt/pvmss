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
	possiblePaths := []string{"../docs", "/app/backend/docs", "../../docs", "./docs"}
	docsDirs := make([]string, 0, len(possiblePaths))
	for _, path := range possiblePaths {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			docsDirs = append(docsDirs, path)
		}
	}
	if len(docsDirs) == 0 {
		t.Skip("Docs directory not found, skipping file existence tests")
	}
	expectedFiles := []string{
		"user.en.md",
		"user.fr.md",
		"admin.en.md",
		"admin.fr.md",
	}
	for _, filename := range expectedFiles {
		found := false
		for _, dir := range docsDirs {
			filePath := filepath.Join(dir, filename)
			if _, err := os.Stat(filePath); err == nil {
				found = true
				break
			}
		}
		assert.True(t, found, "Documentation file %s should exist in at least one docs directory", filename)
	}
}

// TestMakeDocsHandler tests that DocsHandler can be created
func TestMakeDocsHandler(t *testing.T) {
	handler := handlers.MakeDocsHandler()
	assert.NotNil(t, handler, "DocsHandler should be created successfully")
}
