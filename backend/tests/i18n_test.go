package tests

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// findI18nDirectory finds the i18n directory
func findI18nDirectory() string {
	possiblePaths := []string{
		"../i18n",           // From backend/tests/
		"./i18n",            // From backend/
		"/app/backend/i18n", // Container path
	}

	for _, path := range possiblePaths {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return path
		}
	}
	return ""
}

// TestI18nDirectoryExists tests that the i18n directory exists
func TestI18nDirectoryExists(t *testing.T) {
	i18nDir := findI18nDirectory()
	assert.NotEmpty(t, i18nDir, "i18n directory should exist")
}

// getTranslationPairs returns all translation file pairs (*.en.toml with matching *.fr.toml)
func getTranslationPairs(t *testing.T, i18nDir string) []struct{ name, enPath, frPath string } {
	entries, err := os.ReadDir(i18nDir)
	require.NoError(t, err)

	pairs := []struct{ name, enPath, frPath string }{}
	enSuffix := ".en.toml"
	frSuffix := ".fr.toml"

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, enSuffix) {
			continue
		}
		base := strings.TrimSuffix(name, enSuffix)
		enPath := filepath.Join(i18nDir, base+enSuffix)
		frPath := filepath.Join(i18nDir, base+frSuffix)
		if _, err := os.Stat(frPath); err == nil {
			pairs = append(pairs, struct{ name, enPath, frPath string }{name: base, enPath: enPath, frPath: frPath})
		} else {
			t.Logf("Missing French translation for %s (expected %s)", base, frPath)
		}
	}

	return pairs
}

// TestTranslationFilesExist tests that expected translation pairs exist
func TestTranslationFilesExist(t *testing.T) {
	i18nDir := findI18nDirectory()
	if i18nDir == "" {
		t.Skip("i18n directory not found")
	}

	pairs := getTranslationPairs(t, i18nDir)
	require.NotEmpty(t, pairs, "There should be at least one translation file pair (*.en.toml/*.fr.toml)")

	// Ensure commonly expected core files exist
	var hasCommon bool
	for _, p := range pairs {
		if p.name == "common" {
			hasCommon = true
			break
		}
	}
	assert.True(t, hasCommon, "common.en.toml/common.fr.toml should exist")
}

// TestTranslationFilesNotEmpty tests that translation files are not empty
func TestTranslationFilesNotEmpty(t *testing.T) {
	i18nDir := findI18nDirectory()
	if i18nDir == "" {
		t.Skip("i18n directory not found")
	}

	pairs := getTranslationPairs(t, i18nDir)
	for _, p := range pairs {
		t.Run(fmt.Sprintf("not-empty-%s", p.name), func(t *testing.T) {
			for _, filePath := range []string{p.enPath, p.frPath} {
				info, err := os.Stat(filePath)
				require.NoError(t, err, "File %s should exist", filepath.Base(filePath))
				assert.Greater(t, info.Size(), int64(50), "File %s should not be empty", filepath.Base(filePath))
			}
		})
	}
}

// TestTranslationFilesValidTOML tests that translation files are valid TOML
func TestTranslationFilesValidTOML(t *testing.T) {
	i18nDir := findI18nDirectory()
	if i18nDir == "" {
		t.Skip("i18n directory not found")
	}

	pairs := getTranslationPairs(t, i18nDir)
	for _, p := range pairs {
		t.Run(fmt.Sprintf("valid-toml-%s", p.name), func(t *testing.T) {
			for _, filePath := range []string{p.enPath, p.frPath} {
				var data map[string]interface{}
				_, err := toml.DecodeFile(filePath, &data)
				assert.NoError(t, err, "File %s should be valid TOML", filepath.Base(filePath))
				assert.NotEmpty(t, data, "File %s should contain translations", filepath.Base(filePath))
			}
		})
	}
}

// extractTranslationKeys extracts all translation keys from a TOML file
func extractTranslationKeys(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	var keys []string
	keyRegex := regexp.MustCompile(`^\["([^"]+)"\]`)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if matches := keyRegex.FindStringSubmatch(line); len(matches) > 1 {
			keys = append(keys, matches[1])
		}
	}

	return keys, scanner.Err()
}

// TestTranslationKeysConsistency tests that all languages have the same keys
func TestTranslationKeysConsistency(t *testing.T) {
	i18nDir := findI18nDirectory()
	if i18nDir == "" {
		t.Skip("i18n directory not found")
	}

	pairs := getTranslationPairs(t, i18nDir)
	for _, p := range pairs {
		t.Run(fmt.Sprintf("keys-consistency-%s", p.name), func(t *testing.T) {
			enKeys, err := extractTranslationKeys(p.enPath)
			require.NoError(t, err, "Should be able to extract English keys for %s", p.name)

			frKeys, err := extractTranslationKeys(p.frPath)
			require.NoError(t, err, "Should be able to extract French keys for %s", p.name)

			assert.NotEmpty(t, enKeys, "English file should have translation keys for %s", p.name)
			assert.NotEmpty(t, frKeys, "French file should have translation keys for %s", p.name)

			enKeyMap := make(map[string]bool)
			for _, key := range enKeys {
				enKeyMap[key] = true
			}

			frKeyMap := make(map[string]bool)
			for _, key := range frKeys {
				frKeyMap[key] = true
			}

			var missingInFrench []string
			for key := range enKeyMap {
				if !frKeyMap[key] {
					missingInFrench = append(missingInFrench, key)
				}
			}

			var extraInFrench []string
			for key := range frKeyMap {
				if !enKeyMap[key] {
					extraInFrench = append(extraInFrench, key)
				}
			}

			if len(missingInFrench) > 0 {
				t.Logf("[%s] Keys missing in French: %v", p.name, missingInFrench)
			}
			if len(extraInFrench) > 0 {
				t.Logf("[%s] Extra keys in French: %v", p.name, extraInFrench)
			}

			assert.Empty(t, missingInFrench, "[%s] French translation should have all English keys", p.name)
			assert.Empty(t, extraInFrench, "[%s] French translation should not have extra keys", p.name)
		})
	}
}

// TestTranslationFileSizes tests that translation files have similar sizes
func TestTranslationFileSizes(t *testing.T) {
	i18nDir := findI18nDirectory()
	if i18nDir == "" {
		t.Skip("i18n directory not found")
	}

	pairs := getTranslationPairs(t, i18nDir)
	for _, p := range pairs {
		t.Run(fmt.Sprintf("size-ratio-%s", p.name), func(t *testing.T) {
			enInfo, err := os.Stat(p.enPath)
			require.NoError(t, err, "English file should exist for %s", p.name)

			frInfo, err := os.Stat(p.frPath)
			require.NoError(t, err, "French file should exist for %s", p.name)

			enSize := enInfo.Size()
			frSize := frInfo.Size()

			ratio := float64(frSize) / float64(enSize)
			t.Logf("[%s] EN: %d bytes, FR: %d bytes, ratio: %.2f", p.name, enSize, frSize, ratio)

			// Loosen bounds slightly for very small files
			lower := 0.5
			upper := 2.0
			if enSize < 400 && frSize < 400 {
				lower = 0.3
				upper = 3.5
			}
			assert.Greater(t, ratio, lower, "[%s] French file should not be too small compared to English", p.name)
			assert.Less(t, ratio, upper, "[%s] French file should not be too large compared to English", p.name)
		})
	}
}

// TestTranslationFileFormat tests that translation files follow the expected format
func TestTranslationFileFormat(t *testing.T) {
	i18nDir := findI18nDirectory()
	if i18nDir == "" {
		t.Skip("i18n directory not found")
	}

	pairs := getTranslationPairs(t, i18nDir)
	for _, p := range pairs {
		for _, filePath := range []string{p.enPath, p.frPath} {
			filename := filepath.Base(filePath)
			t.Run(fmt.Sprintf("format-%s", filename), func(t *testing.T) {
				file, err := os.Open(filePath)
				require.NoError(t, err, "Should be able to open %s", filename)
				defer func() { _ = file.Close() }()

				scanner := bufio.NewScanner(file)
				lineNum := 0
				hasKeys := false

				for scanner.Scan() {
					lineNum++
					line := strings.TrimSpace(scanner.Text())

					if line == "" || strings.HasPrefix(line, "#") {
						continue
					}

					if strings.HasPrefix(line, "[\"") && strings.Contains(line, "\"]") {
						hasKeys = true
						assert.Regexp(t, `^\["[A-Za-z0-9._]+"\]$`, line,
							"Line %d in %s should follow key format [\"Key.Name\"]", lineNum, filename)
					} else if strings.Contains(line, "=") && !strings.HasPrefix(line, "#") {
						assert.Regexp(t, `^(other|one|few|many|zero)\s*=\s*['"].*['"]`, line,
							"Line %d in %s should follow value format: other = \"value\"", lineNum, filename)
					}
				}

				assert.NoError(t, scanner.Err(), "Should be able to scan %s", filename)
				assert.True(t, hasKeys, "File %s should contain translation keys", filename)
			})
		}
	}
}

// TestTranslationKeysNaming tests that translation keys follow naming conventions
func TestTranslationKeysNaming(t *testing.T) {
	i18nDir := findI18nDirectory()
	if i18nDir == "" {
		t.Skip("i18n directory not found")
	}

	pairs := getTranslationPairs(t, i18nDir)
	for _, p := range pairs {
		t.Run(fmt.Sprintf("naming-%s", p.name), func(t *testing.T) {
			keys, err := extractTranslationKeys(p.enPath)
			require.NoError(t, err, "Should be able to extract keys for %s", p.name)

			for _, key := range keys {
				parts := strings.Split(key, ".")

				if len(parts) >= 2 {
					for i, part := range parts {
						assert.NotEmpty(t, part, "Key %s part %d should not be empty", key, i)
						assert.Regexp(t, `^[A-Z]`, part, "Key %s part '%s' should start with uppercase", key, part)
					}
				} else {
					assert.Regexp(t, `^[A-Z]`, key, "Key %s should start with uppercase", key)
				}
			}
		})
	}
}
