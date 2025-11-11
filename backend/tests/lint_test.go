package tests

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGoFmt(t *testing.T) {
	cmd := exec.Command("go", "fmt", "../...")
	out, err := cmd.CombinedOutput()

	assert.NoError(t, err, "'go fmt' should run without errors")

	if len(out) > 0 {
		// go fmt prints the names of files it has reformatted.
		// If it prints anything, it means some files were not formatted.
		formattedFiles := strings.Split(strings.TrimSpace(string(out)), "\n")
		assert.Fail(t, "The following files were not formatted with 'go fmt':\n"+strings.Join(formattedFiles, "\n"))
	}
}

func TestGoVet(t *testing.T) {
	cmd := exec.Command("go", "vet", "../...")
	out, err := cmd.CombinedOutput()

	assert.NoError(t, err, "'go vet' should run without errors")

	if len(out) > 0 {
		// go vet prints issues to stderr (which is captured by CombinedOutput)
		assert.Fail(t, "'go vet' found issues:\n"+string(out))
	}
}
