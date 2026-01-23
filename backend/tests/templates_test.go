package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// findComponentsDirectory finds the backend/components directory containing Templ templates
func findComponentsDirectory() string {
	possiblePaths := []string{
		"../../components",        // From backend/tests/
		"../components",           // From backend/
		"./components",            // From project root
		"/app/backend/components", // Container path
	}

	// Try relative paths first
	for _, path := range possiblePaths {
		absPath, err := filepath.Abs(path)
		if err == nil {
			if info, err := os.Stat(absPath); err == nil && info.IsDir() {
				return absPath
			}
		}
	}

	// Fallback: try to find it from current working directory
	cwd, err := os.Getwd()
	if err == nil {
		// Walk up the directory tree to find the project root
		for {
			componentsPath := filepath.Join(cwd, "backend", "components")
			if info, err := os.Stat(componentsPath); err == nil && info.IsDir() {
				return componentsPath
			}

			parent := filepath.Dir(cwd)
			if parent == cwd {
				break // Reached root directory
			}
			cwd = parent
		}
	}

	return ""
}

// findFrontendDirectory finds the frontend directory containing static assets
func findFrontendDirectory() string {
	possiblePaths := []string{
		"../../frontend", // From backend/tests/
		"../frontend",    // From backend/
		"./frontend",     // From project root
		"/app/frontend",  // Container path
	}

	// Try relative paths first
	for _, path := range possiblePaths {
		absPath, err := filepath.Abs(path)
		if err == nil {
			if info, err := os.Stat(absPath); err == nil && info.IsDir() {
				return absPath
			}
		}
	}

	// Fallback: try to find it from current working directory
	cwd, err := os.Getwd()
	if err == nil {
		// Walk up the directory tree to find the project root
		for {
			frontendPath := filepath.Join(cwd, "frontend")
			if info, err := os.Stat(frontendPath); err == nil && info.IsDir() {
				return frontendPath
			}

			parent := filepath.Dir(cwd)
			if parent == cwd {
				break // Reached root directory
			}
			cwd = parent
		}
	}

	return ""
}

// TestComponentsDirectoryExists tests that the components directory exists
func TestComponentsDirectoryExists(t *testing.T) {
	componentsDir := findComponentsDirectory()
	assert.NotEmpty(t, componentsDir, "Components directory should exist")
}

// TestTemplComponentFilesExist tests that Templ component files exist
func TestTemplComponentFilesExist(t *testing.T) {
	componentsDir := findComponentsDirectory()
	if componentsDir == "" {
		t.Skip("Components directory not found")
	}

	// Find all .templ files
	var templFiles []string
	err := filepath.Walk(componentsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".templ") {
			templFiles = append(templFiles, path)
		}
		return nil
	})

	require.NoError(t, err, "Should be able to walk components directory")
	assert.NotEmpty(t, templFiles, "Should find at least one .templ file")

	for _, filePath := range templFiles {
		_, statErr := os.Stat(filePath)
		assert.NoError(t, statErr, "Template file %s should exist", filepath.Base(filePath))
	}
}

// TestTemplComponentFilesNotEmpty tests that Templ component files are not empty
func TestTemplComponentFilesNotEmpty(t *testing.T) {
	componentsDir := findComponentsDirectory()
	if componentsDir == "" {
		t.Skip("Components directory not found")
	}

	var templFiles []string
	filepath.Walk(componentsDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(path, ".templ") {
			templFiles = append(templFiles, path)
		}
		return nil
	})

	require.NotEmpty(t, templFiles, "Should find at least one .templ file")

	for _, filePath := range templFiles {
		info, err := os.Stat(filePath)
		require.NoError(t, err, "File %s should exist", filePath)
		assert.Greater(t, info.Size(), int64(10), "File %s should not be empty", filepath.Base(filePath))
	}
}

// TestTemplComponentFilesHaveValidSyntax tests that Templ files have valid syntax
func TestTemplComponentFilesHaveValidSyntax(t *testing.T) {
	componentsDir := findComponentsDirectory()
	if componentsDir == "" {
		t.Skip("Components directory not found")
	}

	var templFiles []string
	filepath.Walk(componentsDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(path, ".templ") {
			templFiles = append(templFiles, path)
		}
		return nil
	})

	for _, filePath := range templFiles {
		content, err := os.ReadFile(filePath)
		require.NoError(t, err, "Should be able to read %s", filePath)

		contentStr := string(content)
		filename := filepath.Base(filePath)

		// Templ files should have package declaration
		assert.True(t, strings.Contains(contentStr, "package "), "File %s should have package declaration", filename)

		// Templ files should have templ keyword
		assert.True(t, strings.Contains(contentStr, "templ "), "File %s should have templ keyword", filename)
	}
}

// TestTemplComponentFileNaming tests that component files follow naming conventions
func TestTemplComponentFileNaming(t *testing.T) {
	componentsDir := findComponentsDirectory()
	if componentsDir == "" {
		t.Skip("Components directory not found")
	}

	var templFiles []string
	filepath.Walk(componentsDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(path, ".templ") {
			templFiles = append(templFiles, path)
		}
		return nil
	})

	for _, filePath := range templFiles {
		filename := filepath.Base(filePath)

		// Templ files should use snake_case
		assert.Regexp(t, `^[a-z][a-z0-9_]*\.templ$`, filename,
			"Component %s should use lowercase with underscores", filename)
	}
}

// TestNoLegacyAdminHTMLTemplates tests that no legacy admin HTML templates remain
// Note: Admin templates have been migrated to Templ components in backend/components/
func TestNoLegacyAdminHTMLTemplates(t *testing.T) {
	frontendDir := findFrontendDirectory()
	if frontendDir == "" {
		t.Skip("Frontend directory not found")
	}

	var htmlFiles []string
	filepath.Walk(frontendDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(path, ".html") {
			htmlFiles = append(htmlFiles, path)
		}
		return nil
	})

	adminTemplates := []string{}
	for _, filePath := range htmlFiles {
		filename := filepath.Base(filePath)
		if strings.HasPrefix(filename, "admin_") {
			adminTemplates = append(adminTemplates, filename)
		}
	}

	// Admin templates have been migrated to Templ components
	// No legacy admin HTML templates should remain
	assert.Empty(t, adminTemplates, "Legacy admin HTML templates should be removed (migrated to Templ)")
	t.Logf("Found %d legacy admin templates (expected 0)", len(adminTemplates))
}

// TestFrontendStaticAssetsExist tests that frontend static asset directories exist
func TestFrontendStaticAssetsExist(t *testing.T) {
	frontendDir := findFrontendDirectory()
	if frontendDir == "" {
		t.Skip("Frontend directory not found")
	}

	// Check CSS directory
	cssDir := filepath.Join(frontendDir, "css")
	info, err := os.Stat(cssDir)
	assert.NoError(t, err, "CSS directory should exist")
	if err == nil {
		assert.True(t, info.IsDir(), "css should be a directory")
	}

	// Check JS directory
	jsDir := filepath.Join(frontendDir, "js")
	info, err = os.Stat(jsDir)
	assert.NoError(t, err, "JS directory should exist")
	if err == nil {
		assert.True(t, info.IsDir(), "js should be a directory")
	}
}

// TestGeneratedTemplFilesExist tests that generated Go files from Templ exist
func TestGeneratedTemplFilesExist(t *testing.T) {
	componentsDir := findComponentsDirectory()
	if componentsDir == "" {
		t.Skip("Components directory not found")
	}

	var templFiles []string
	filepath.Walk(componentsDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(path, ".templ") {
			templFiles = append(templFiles, path)
		}
		return nil
	})

	require.NotEmpty(t, templFiles, "Should find at least one .templ file")

	// For each .templ file, check that a corresponding _templ.go file exists
	for _, templPath := range templFiles {
		// Replace .templ with _templ.go
		generatedPath := strings.TrimSuffix(templPath, ".templ") + "_templ.go"
		info, err := os.Stat(generatedPath)
		assert.NoError(t, err, "Generated file %s should exist for %s", filepath.Base(generatedPath), filepath.Base(templPath))
		if err == nil {
			assert.False(t, info.IsDir(), "Generated file %s should not be a directory", filepath.Base(generatedPath))
		}
	}
}
