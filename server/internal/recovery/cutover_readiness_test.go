// cutover_readiness_test.go asserts the post-cutover state of deployment
// manifests and the Dockerfile: they must reference only v0.4 paths
// (server/, web/), never legacy paths (backend/, frontend/).
//
// T028 authored this test as a pre-cutover canary (asserting legacy
// entrypoints were still present); T032 flipped it to assert the
// post-cutover state after T029-T031 updated the manifests. The net
// result is a live test that fails loudly if someone reverts the
// manifests to legacy paths.
package recovery_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// findRepoRootForCutover finds the repository root by walking up from cwd.
func findRepoRootForCutover(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	for range 10 {
		if _, err := os.Stat(filepath.Join(dir, "Makefile")); err == nil {
			if _, err := os.Stat(filepath.Join(dir, "server", "cmd")); err == nil {
				return dir
			}
		}

		dir = filepath.Dir(dir)
	}

	t.Fatal("could not find repo root")

	return ""
}

// TestCutoverReadiness_DeploymentManifestReferencesV04 asserts that
// pvmss-deployment.yaml references the v0.4 binary entrypoint and web
// build path, not the legacy pvmss-backend / frontend paths (FR-009).
func TestCutoverReadiness_DeploymentManifestReferencesV04(t *testing.T) {
	t.Parallel()

	repoRoot := findRepoRootForCutover(t)
	manifest := filepath.Join(repoRoot, "pvmss-deployment.yaml")

	content, err := os.ReadFile(manifest) //nolint:gosec // test reads known fixture path
	if err != nil {
		t.Fatalf("read %s: %v", manifest, err)
	}

	text := string(content)

	// Must NOT reference the legacy binary name
	if strings.Contains(text, "pvmss-backend") {
		t.Error("pvmss-deployment.yaml still references legacy binary 'pvmss-backend'")
	}
	// Must NOT reference the legacy frontend path
	if strings.Contains(text, "/app/frontend") {
		t.Error("pvmss-deployment.yaml still references legacy path '/app/frontend'")
	}
	// Must reference the v0.4 binary
	if !strings.Contains(text, "pvmss") {
		t.Error("pvmss-deployment.yaml does not reference the v0.4 binary")
	}
}

// TestCutoverReadiness_HelmChartReferencesV04 asserts that helm/ templates
// and values do not reference legacy paths (FR-009).
func TestCutoverReadiness_HelmChartReferencesV04(t *testing.T) {
	t.Parallel()

	repoRoot := findRepoRootForCutover(t)
	helmDir := filepath.Join(repoRoot, "helm")

	// Walk all yaml files under helm/
	err := filepath.Walk(helmDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}

		if !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml") {
			return nil
		}

		content, err := os.ReadFile(path) //nolint:gosec // test walks known helm fixture paths
		if err != nil {
			return err
		}

		text := string(content)
		if strings.Contains(text, "pvmss-backend") {
			t.Errorf("%s still references legacy binary 'pvmss-backend'", path)
		}

		if strings.Contains(text, "/app/frontend") {
			t.Errorf("%s still references legacy path '/app/frontend'", path)
		}

		return nil
	})
	if err != nil {
		t.Fatalf("walk helm/: %v", err)
	}
}

// TestCutoverReadiness_DockerfileBuildsOnlyV04 asserts that the Dockerfile
// does not compile or copy backend/ or frontend/ — only server/ and web/
// (FR-010).
func TestCutoverReadiness_DockerfileBuildsOnlyV04(t *testing.T) {
	t.Parallel()

	repoRoot := findRepoRootForCutover(t)
	dockerfile := filepath.Join(repoRoot, "Dockerfile")

	content, err := os.ReadFile(dockerfile) //nolint:gosec // test reads known fixture path
	if err != nil {
		t.Fatalf("read %s: %v", dockerfile, err)
	}

	text := string(content)

	// Must NOT copy or build backend/ or frontend/
	legacyRefRe := regexp.MustCompile(`(?i)(COPY|cd|go build).*\b(backend|frontend)\b`)
	for line := range strings.SplitSeq(text, "\n") {
		if legacyRefRe.MatchString(line) {
			t.Errorf("Dockerfile line references legacy code: %s", strings.TrimSpace(line))
		}
	}

	// Must copy/build server/ and web/
	if !strings.Contains(text, "server/") {
		t.Error("Dockerfile does not reference server/")
	}

	if !strings.Contains(text, "web/") {
		t.Error("Dockerfile does not reference web/")
	}
}

// TestCutoverReadiness_NoLegacyEntryPoint asserts the Dockerfile ENTRYPOINT
// references the v0.4 binary, not the legacy pvmss-backend.
func TestCutoverReadiness_NoLegacyEntryPoint(t *testing.T) {
	t.Parallel()

	repoRoot := findRepoRootForCutover(t)
	dockerfile := filepath.Join(repoRoot, "Dockerfile")

	content, err := os.ReadFile(dockerfile) //nolint:gosec // test reads known fixture path
	if err != nil {
		t.Fatalf("read %s: %v", dockerfile, err)
	}

	text := string(content)

	for line := range strings.SplitSeq(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "ENTRYPOINT") {
			if strings.Contains(trimmed, "pvmss-backend") {
				t.Errorf("Dockerfile ENTRYPOINT still uses legacy binary: %s", trimmed)
			}

			if strings.Contains(trimmed, "/app/frontend") {
				t.Errorf("Dockerfile ENTRYPOINT still uses legacy frontend path: %s", trimmed)
			}
		}
	}
}
