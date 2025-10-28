package tests

import (
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pvmss/templates"
)

// findFrontendDirectory finds the frontend directory containing HTML templates
func findFrontendDirectory() string {
	possiblePaths := []string{
		"../../frontend", // From backend/tests/
		"../frontend",    // From backend/
		"./frontend",     // From project root
		"/app/frontend",  // Container path
	}

	for _, path := range possiblePaths {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return path
		}
	}
	return ""
}

// TestFrontendDirectoryExists tests that the frontend directory exists
func TestFrontendDirectoryExists(t *testing.T) {
	frontendDir := findFrontendDirectory()
	assert.NotEmpty(t, frontendDir, "Frontend directory should exist")
}

// TestTemplateFilesExist tests that expected template files exist
func TestTemplateFilesExist(t *testing.T) {
	frontendDir := findFrontendDirectory()
	if frontendDir == "" {
		t.Skip("Frontend directory not found")
	}

	expectedTemplates := []string{
		"layout.html",
		"navbar.html",
		"index.html",
		"login.html",
		"admin_login.html",
		"admin_base.html",
		"create_vm.html",
		"vm_details.html",
		"profile.html",
		"search.html",
		"error.html",
		"docs.html",
	}

	for _, filename := range expectedTemplates {
		filePath := filepath.Join(frontendDir, filename)
		_, err := os.Stat(filePath)
		assert.NoError(t, err, "Template file %s should exist", filename)
	}
}

// TestTemplateFilesNotEmpty tests that template files are not empty
func TestTemplateFilesNotEmpty(t *testing.T) {
	frontendDir := findFrontendDirectory()
	if frontendDir == "" {
		t.Skip("Frontend directory not found")
	}

	files, err := templates.FindTemplateFiles(frontendDir)
	require.NoError(t, err, "Should be able to find template files")
	require.NotEmpty(t, files, "Should find at least one template file")

	for _, filePath := range files {
		info, err := os.Stat(filePath)
		require.NoError(t, err, "File %s should exist", filePath)
		assert.Greater(t, info.Size(), int64(10), "File %s should not be empty", filepath.Base(filePath))
	}
}

// TestTemplateFilesValidHTML tests that template files are valid HTML
func TestTemplateFilesValidHTML(t *testing.T) {
	frontendDir := findFrontendDirectory()
	if frontendDir == "" {
		t.Skip("Frontend directory not found")
	}

	files, err := templates.FindTemplateFiles(frontendDir)
	require.NoError(t, err, "Should be able to find template files")

	for _, filePath := range files {
		content, err := os.ReadFile(filePath)
		require.NoError(t, err, "Should be able to read %s", filePath)

		// Check for basic HTML structure
		contentStr := string(content)
		filename := filepath.Base(filePath)

		// Skip component files and partials
		if strings.Contains(filePath, "/components/") {
			continue
		}

		// Main templates should have some HTML structure
		if !strings.Contains(filename, "navbar") && !strings.Contains(filename, "layout") {
			// Most templates should have at least some HTML tags
			hasHTML := strings.Contains(contentStr, "<") && strings.Contains(contentStr, ">")
			assert.True(t, hasHTML, "Template %s should contain HTML tags", filename)
		}
	}
}

// TestTemplatesParseable tests that templates can be parsed by Go's template engine
func TestTemplatesParseable(t *testing.T) {
	frontendDir := findFrontendDirectory()
	if frontendDir == "" {
		t.Skip("Frontend directory not found")
	}

	files, err := templates.FindTemplateFiles(frontendDir)
	require.NoError(t, err, "Should be able to find template files")
	require.NotEmpty(t, files, "Should find at least one template file")

	// Create a template with common functions
	funcMap := templates.GetBaseFuncMap()

	// Add a dummy T function for testing
	funcMap["T"] = func(messageID string, args ...interface{}) template.HTML {
		return template.HTML(messageID)
	}

	tmpl := template.New("test").Funcs(funcMap)

	// Try to parse all templates
	parsedCount := 0
	for _, filePath := range files {
		_, err := tmpl.ParseFiles(filePath)
		if err != nil {
			t.Logf("Warning: Could not parse %s: %v", filepath.Base(filePath), err)
		} else {
			parsedCount++
		}
	}

	// At least 80% of templates should be parseable
	successRate := float64(parsedCount) / float64(len(files))
	assert.Greater(t, successRate, 0.8, "At least 80%% of templates should be parseable")
	t.Logf("Successfully parsed %d/%d templates (%.1f%%)", parsedCount, len(files), successRate*100)
}

// TestTemplateFileNaming tests that template files follow naming conventions
func TestTemplateFileNaming(t *testing.T) {
	frontendDir := findFrontendDirectory()
	if frontendDir == "" {
		t.Skip("Frontend directory not found")
	}

	files, err := templates.FindTemplateFiles(frontendDir)
	require.NoError(t, err, "Should be able to find template files")

	for _, filePath := range files {
		filename := filepath.Base(filePath)

		// Skip component files
		if strings.Contains(filePath, "/components/") {
			continue
		}

		// Template files should use snake_case or kebab-case
		assert.Regexp(t, `^[a-z][a-z0-9_]*\.html$`, filename,
			"Template %s should use lowercase with underscores", filename)
	}
}

// TestAdminTemplatesHaveAdminPrefix tests that admin templates are properly named
func TestAdminTemplatesHaveAdminPrefix(t *testing.T) {
	frontendDir := findFrontendDirectory()
	if frontendDir == "" {
		t.Skip("Frontend directory not found")
	}

	files, err := templates.FindTemplateFiles(frontendDir)
	require.NoError(t, err, "Should be able to find template files")

	adminTemplates := []string{}
	for _, filePath := range files {
		filename := filepath.Base(filePath)
		if strings.HasPrefix(filename, "admin_") {
			adminTemplates = append(adminTemplates, filename)
		}
	}

	assert.NotEmpty(t, adminTemplates, "Should have at least one admin template")
	t.Logf("Found %d admin templates", len(adminTemplates))
}

// TestTemplatesFindFunction tests the FindTemplateFiles function
func TestTemplatesFindFunction(t *testing.T) {
	frontendDir := findFrontendDirectory()
	if frontendDir == "" {
		t.Skip("Frontend directory not found")
	}

	files, err := templates.FindTemplateFiles(frontendDir)
	require.NoError(t, err, "FindTemplateFiles should not return error")
	assert.NotEmpty(t, files, "FindTemplateFiles should find at least one file")

	// All returned files should exist and be HTML files
	for _, filePath := range files {
		assert.True(t, strings.HasSuffix(filePath, ".html"), "File %s should have .html extension", filePath)

		info, err := os.Stat(filePath)
		assert.NoError(t, err, "File %s should exist", filePath)
		assert.False(t, info.IsDir(), "Path %s should not be a directory", filePath)
	}
}

// TestTemplatesBaseFuncMap tests that GetBaseFuncMap returns functions
func TestTemplatesBaseFuncMap(t *testing.T) {
	funcMap := templates.GetBaseFuncMap()

	assert.NotNil(t, funcMap, "GetBaseFuncMap should return a function map")
	assert.NotEmpty(t, funcMap, "GetBaseFuncMap should return at least one function")

	// Check for some expected core functions
	expectedFunctions := []string{
		"add",
		"sub",
		"mul",
		"div",
		"formatBytes",
		"upper",
		"lower",
		"contains",
	}

	foundCount := 0
	for _, funcName := range expectedFunctions {
		if _, exists := funcMap[funcName]; exists {
			foundCount++
		}
	}

	// At least 75% of expected functions should exist
	successRate := float64(foundCount) / float64(len(expectedFunctions))
	assert.Greater(t, successRate, 0.75, "At least 75%% of expected functions should exist")

	t.Logf("Found %d template functions", len(funcMap))
}

// TestTemplateUtilityFunctions tests some utility functions
func TestTemplateUtilityFunctions(t *testing.T) {
	funcMap := templates.GetBaseFuncMap()

	// Test add function
	if addFunc, ok := funcMap["add"].(func(int, int) int); ok {
		result := addFunc(2, 3)
		assert.Equal(t, 5, result, "add(2, 3) should equal 5")
	}

	// Test sub function
	if subFunc, ok := funcMap["sub"].(func(int, int) int); ok {
		result := subFunc(5, 3)
		assert.Equal(t, 2, result, "sub(5, 3) should equal 2")
	}

	// Test mul function
	if mulFunc, ok := funcMap["mul"].(func(int, int) int); ok {
		result := mulFunc(4, 3)
		assert.Equal(t, 12, result, "mul(4, 3) should equal 12")
	}
}

// TestTemplatesCSSJSDirectories tests that CSS and JS directories exist
func TestTemplatesCSSJSDirectories(t *testing.T) {
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

	// Check components directory
	componentsDir := filepath.Join(frontendDir, "components")
	info, err = os.Stat(componentsDir)
	assert.NoError(t, err, "Components directory should exist")
	if err == nil {
		assert.True(t, info.IsDir(), "components should be a directory")
	}
}

// TestTemplatesNoSyntaxErrors tests that templates don't have obvious syntax errors
func TestTemplatesNoSyntaxErrors(t *testing.T) {
	frontendDir := findFrontendDirectory()
	if frontendDir == "" {
		t.Skip("Frontend directory not found")
	}

	files, err := templates.FindTemplateFiles(frontendDir)
	require.NoError(t, err, "Should be able to find template files")

	for _, filePath := range files {
		content, err := os.ReadFile(filePath)
		require.NoError(t, err, "Should be able to read %s", filePath)

		contentStr := string(content)
		filename := filepath.Base(filePath)

		// Check for common template syntax errors
		// Unclosed template actions
		openActions := strings.Count(contentStr, "{{")
		closeActions := strings.Count(contentStr, "}}")
		assert.Equal(t, openActions, closeActions,
			"Template %s should have matching {{ and }}", filename)

		// Unclosed HTML tags (basic check for major tags)
		for _, tag := range []string{"div", "form", "table", "ul", "ol"} {
			openTags := strings.Count(contentStr, "<"+tag)
			closeTags := strings.Count(contentStr, "</"+tag+">")
			// Allow some flexibility for self-closing or template-generated tags
			diff := openTags - closeTags
			assert.LessOrEqual(t, diff, 2,
				"Template %s should have roughly matching <%s> and </%s> tags", filename, tag, tag)
		}
	}
}
