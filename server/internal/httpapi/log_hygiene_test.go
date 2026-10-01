package httpapi

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Errors are logged once, by the access log: a handler must not log at Error
// and then answer with a 4xx/5xx (use SetError/SetErrorMsg instead). The
// health handler is the one deliberate exception: /health is capped at Debug
// in the access log, so its failures must be logged directly.
func TestHandlersDoNotLogErrorThenFailTheRequest(t *testing.T) {
	t.Parallel()

	logErr := regexp.MustCompile(`\.(log|Log)\.Error\(|\)\.ErrorContext\(`)
	failing := regexp.MustCompile(`Status(InternalServerError|BadGateway|ServiceUnavailable|GatewayTimeout|BadRequest|Unauthorized|Forbidden|NotFound|Conflict|TooManyRequests)`)

	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}

	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") || f == "health.go" {
			continue
		}

		src, err := os.ReadFile(f) //nolint:gosec // reads repo source files
		if err != nil {
			t.Fatal(err)
		}

		lines := strings.Split(string(src), "\n")
		for i, l := range lines {
			if !logErr.MatchString(l) {
				continue
			}

			for _, next := range lines[i+1 : min(i+7, len(lines))] {
				trimmed := strings.TrimSpace(next)
				if failing.MatchString(next) {
					t.Errorf("%s:%d logs at Error then writes a failing status; use SetErrorMsg", f, i+1)
				}

				if strings.HasPrefix(trimmed, "return") || trimmed == "}" {
					break
				}
			}
		}
	}
}
