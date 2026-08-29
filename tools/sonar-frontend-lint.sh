#!/bin/sh

# Run ESLint on web/ (including .svelte files) and convert the results to
# SonarQube generic issue format for import as external issues.
#
# SonarQube Community Build cannot parse .svelte files, but eslint-plugin-svelte
# can. This script bridges the gap: ESLint checks all .svelte/.ts/.js files,
# and the results are imported into SonarQube via sonar.externalIssuesReportPaths.
#
# Output file:
#   .sonar/web-eslint.json

set -u

mkdir -p .sonar

REPO_ROOT=$(pwd)

# Convert ESLint JSON output to SonarQube generic issue format.
# Args: $1 = output file, $2 = input ESLint JSON file, $3 = prefix to strip
convert_eslint_to_sonar() {
    out_file="$1"
    in_file="$2"
    prefix="$3"
    python3 "$REPO_ROOT/tools/eslint-to-sonar.py" "$in_file" "$out_file" "$prefix"
}

echo "Running ESLint on web/..."
cd web || exit 1
bun run lint -- -f json 2>/dev/null > /tmp/eslint-web.json
cd "$REPO_ROOT" || exit 1
convert_eslint_to_sonar ".sonar/web-eslint.json" /tmp/eslint-web.json "$REPO_ROOT/web"

rm -f /tmp/eslint-web.json

echo "Running vitest coverage on web/..."
cd web || exit 1
bun run test:coverage >/dev/null 2>&1 || echo "Warning: vitest coverage reported failures (report still generated)." >&2
cd "$REPO_ROOT" || exit 1
if [ -f web/coverage/lcov.info ]; then
    # Vitest emits paths relative to web/ (e.g. "src/app.css"); SonarQube's
    # base dir is the repo root and sources are under web/src, so prefix
    # every SF: line with "web/".
    # Drop files SonarQube does not index: .svelte/.svelte.ts/.svelte.js
    # (excluded via sonar.javascript.exclusions), generated paraglide .js,
    # and node_modules — otherwise SonarQube logs thousands of unresolved
    # path warnings and the coverage sensor stalls.
    sed 's|^SF:src/|SF:web/src/|' web/coverage/lcov.info \
        | grep -v -E '^SF:.*\.(svelte|svelte\.ts|svelte\.js)$' \
        | grep -v -E '^SF:web/src/lib/paraglide/' \
        > .sonar/web-lcov.info
else
    echo "Warning: web/coverage/lcov.info not found; web coverage will be 0%." >&2
    : > .sonar/web-lcov.info
fi

echo "ESLint + coverage reports ready in .sonar/"
