package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestProjectStructure tests that key directories and files exist
func TestProjectStructure(t *testing.T) {
	// Get the root directory (backend)
	rootDir := ".."

	criticalDirs := []string{
		"handlers",
		"templates",
		"i18n",
		"proxmox",
		"security",
		"state",
		"middleware",
		"constants",
		"utils",
		"logger",
		"tests",
	}

	for _, dir := range criticalDirs {
		dirPath := filepath.Join(rootDir, dir)
		info, err := os.Stat(dirPath)
		assert.NoError(t, err, "Directory %s should exist", dir)
		if err == nil {
			assert.True(t, info.IsDir(), "%s should be a directory", dir)
		}
	}
}

// TestCriticalFiles tests that critical files exist
func TestCriticalFiles(t *testing.T) {
	rootDir := ".."

	criticalFiles := []string{
		"go.mod",
		"go.sum",
		"main.go",
	}

	for _, file := range criticalFiles {
		filePath := filepath.Join(rootDir, file)
		info, err := os.Stat(filePath)
		assert.NoError(t, err, "File %s should exist", file)
		if err == nil {
			assert.False(t, info.IsDir(), "%s should be a file", file)
			assert.Greater(t, info.Size(), int64(0), "%s should not be empty", file)
		}
	}
}

// TestI18nFilesStructure tests that i18n files follow naming convention
func TestI18nFilesStructure(t *testing.T) {
	i18nDir := "../i18n"

	info, err := os.Stat(i18nDir)
	if err != nil {
		t.Skip("i18n directory not found")
	}

	assert.True(t, info.IsDir(), "i18n should be a directory")

	// Check that we have both .en.toml and .fr.toml files
	entries, err := os.ReadDir(i18nDir)
	assert.NoError(t, err, "Should be able to read i18n directory")

	enFiles := 0
	frFiles := 0

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if filepath.Ext(name) == ".toml" {
			if len(name) >= 8 && name[len(name)-8:] == ".en.toml" {
				enFiles++
			}
			if len(name) >= 8 && name[len(name)-8:] == ".fr.toml" {
				frFiles++
			}
		}
	}

	assert.Greater(t, enFiles, 0, "Should have at least one English translation file")
	assert.Greater(t, frFiles, 0, "Should have at least one French translation file")
	assert.Equal(t, enFiles, frFiles, "Should have same number of EN and FR translation files")
}

// TestHandlersStructure tests that handlers directory is properly organized
func TestHandlersStructure(t *testing.T) {
	handlersDir := "../handlers"

	info, err := os.Stat(handlersDir)
	if err != nil {
		t.Skip("handlers directory not found")
	}

	assert.True(t, info.IsDir(), "handlers should be a directory")

	// Check for key handler files
	keyHandlers := []string{
		"admin.go",
		"profile.go",
		"vm_create.go",
		"vm_details_base.go",
		"vm_details_helpers.go",
		"vm_details_info.go",
		"vm_details_metrics.go",
		"vm_details_validation.go",
		"vm_actions.go",
		"auth.go",
		"helpers.go",
	}

	for _, handler := range keyHandlers {
		handlerPath := filepath.Join(handlersDir, handler)
		info, err := os.Stat(handlerPath)
		assert.NoError(t, err, "Handler file %s should exist", handler)
		if err == nil {
			assert.False(t, info.IsDir(), "%s should be a file", handler)
			assert.Greater(t, info.Size(), int64(100), "%s should have substantial content", handler)
		}
	}
}

// TestTemplatesStructure tests that templates directory is properly organized
func TestTemplatesStructure(t *testing.T) {
	templatesDir := "../templates"

	info, err := os.Stat(templatesDir)
	if err != nil {
		t.Skip("templates directory not found")
	}

	assert.True(t, info.IsDir(), "templates should be a directory")

	// Check for key template files
	keyTemplates := []string{
		"base.go",
		"formatting.go",
		"math_utils.go",
		"string_utils.go",
		"collections.go",
	}

	for _, template := range keyTemplates {
		templatePath := filepath.Join(templatesDir, template)
		info, err := os.Stat(templatePath)
		assert.NoError(t, err, "Template file %s should exist", template)
		if err == nil {
			assert.False(t, info.IsDir(), "%s should be a file", template)
			assert.Greater(t, info.Size(), int64(50), "%s should have content", template)
		}
	}
}

// TestProxmoxStructure tests that proxmox package is properly organized
func TestProxmoxStructure(t *testing.T) {
	proxmoxDir := "../proxmox"

	info, err := os.Stat(proxmoxDir)
	if err != nil {
		t.Skip("proxmox directory not found")
	}

	assert.True(t, info.IsDir(), "proxmox should be a directory")

	// Check for key proxmox files
	keyFiles := []string{
		"resty_client.go",
		"telmate_client.go",
		"interfaces.go",
	}

	for _, file := range keyFiles {
		filePath := filepath.Join(proxmoxDir, file)
		info, err := os.Stat(filePath)
		assert.NoError(t, err, "Proxmox file %s should exist", file)
		if err == nil {
			assert.False(t, info.IsDir(), "%s should be a file", file)
			assert.Greater(t, info.Size(), int64(100), "%s should have content", file)
		}
	}
}

// TestStateStructure tests that state package is properly organized
func TestStateStructure(t *testing.T) {
	stateDir := "../state"

	info, err := os.Stat(stateDir)
	if err != nil {
		t.Skip("state directory not found")
	}

	assert.True(t, info.IsDir(), "state should be a directory")

	// Check for key state files
	keyFiles := []string{
		"interface.go",
		"manager.go",
		"settings.go",
	}

	for _, file := range keyFiles {
		filePath := filepath.Join(stateDir, file)
		info, err := os.Stat(filePath)
		assert.NoError(t, err, "State file %s should exist", file)
		if err == nil {
			assert.False(t, info.IsDir(), "%s should be a file", file)
			assert.Greater(t, info.Size(), int64(100), "%s should have content", file)
		}
	}
}
