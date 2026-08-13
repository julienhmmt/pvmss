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
echo "ESLint reports ready in .sonar/"
