package store

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"time"
)

// QuarantineDatabase renames the SQLite file at path, and its -wal/-shm
// companions, to "<path>.incompatible-<UTC time>" so a fresh database can be
// created in its place. Nothing is deleted. It returns the new main-file path.
func QuarantineDatabase(path string, now time.Time) (string, error) {
	if path == "" {
		return "", errors.New("database path is empty")
	}

	suffix := ".incompatible-" + now.UTC().Format("20060102T150405Z")
	target := path + suffix

	if _, err := os.Stat(target); err == nil {
		return "", fmt.Errorf("quarantine target %q already exists", target)
	}

	if err := os.Rename(path, target); err != nil {
		return "", fmt.Errorf("move database aside: %w", err)
	}

	for _, ext := range []string{"-wal", "-shm"} {
		if err := os.Rename(path+ext, target+ext); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("move database %s file aside: %w", ext, err)
		}
	}

	return target, nil
}
