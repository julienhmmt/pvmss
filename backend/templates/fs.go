package templates

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

// FindTemplateFiles walks the given root directory and returns a slice of absolute paths
// to all files with the .html extension.
func FindTemplateFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".html") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk template directory %s: %w", root, err)
	}
	return files, nil
}
