package templates

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"pvmss/logger"
)

// FindTemplateFiles walks the given root directory and returns a slice of absolute paths
// to all files with the .html extension.
func FindTemplateFiles(root string) ([]string, error) {
	logger.Get().Debug().Str("root", root).Msg("Scanning for template files")

	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			logger.Get().Error().Err(err).Str("path", path).Msg("Error walking template directory")
			return err
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".html") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		logger.Get().Error().Err(err).Str("root", root).Msg("Failed to find template files")
		return nil, fmt.Errorf("failed to walk template directory %s: %w", root, err)
	}

	logger.Get().Info().Int("count", len(files)).Str("root", root).Msg("Template files found")
	return files, nil
}
