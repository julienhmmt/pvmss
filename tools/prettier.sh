#!/bin/sh

# Format Markdown, TypeScript, Svelte, and CSS files with Prettier for PVMSS project
echo "Formatting Markdown, TypeScript, Svelte, and CSS files with Prettier..."

# Check if any files exist (excluding node_modules, build, .svelte-kit)
md_count=$(find . -name '*.md' -type f -not -path '*/node_modules/*' -not -path '*/.svelte-kit/*' -not -path '*/build/*' 2>/dev/null | wc -l)
ts_count=$(find . -name '*.ts' -type f -not -path '*/node_modules/*' -not -path '*/.svelte-kit/*' -not -path '*/build/*' 2>/dev/null | wc -l)
svelte_count=$(find . -name '*.svelte' -type f -not -path '*/node_modules/*' -not -path '*/.svelte-kit/*' -not -path '*/build/*' 2>/dev/null | wc -l)
css_count=$(find . -name '*.css' -type f -not -path '*/node_modules/*' -not -path '*/.svelte-kit/*' -not -path '*/build/*' 2>/dev/null | wc -l)
total_count=$((md_count + ts_count + svelte_count + css_count))

if [ "$total_count" -eq 0 ]; then
    echo "⚠️  No Markdown, TypeScript, Svelte, or CSS files found in project"
    exit 0
fi

echo "✅ Found files:"
echo "  Markdown: $md_count files"
echo "  TypeScript: $ts_count files"
echo "  Svelte: $svelte_count files"
echo "  CSS: $css_count files"
echo "  Total: $total_count files"

# Run Prettier and capture output
echo "Running Prettier to format files..."
output=$(docker run --rm -v .:/workspace -w /workspace node:lts-alpine sh -c "npm install -g prettier prettier-plugin-svelte && prettier --write '**/*.{md,ts,tsx,svelte,css}' --ignore 'node_modules/**' --ignore '.svelte-kit/**' --ignore 'build/**' --plugin prettier-plugin-svelte 2>&1" || true)

if [ $? -eq 0 ]; then
    echo "✅ Prettier formatting completed successfully!"
    
    # Show only changed files (filter out unchanged and installation noise)
    changed_files=$(echo "$output" | grep -E '\.(md|ts|tsx|svelte|css)' | grep -v 'unchanged' | grep -v 'node_modules' | grep -v '.svelte-kit' | grep -v 'build')
    
    if [ -n "$changed_files" ]; then
        echo "Files changed:"
        echo "$changed_files"
        changed_count=$(echo "$changed_files" | wc -l)
        echo "Total: $changed_count / $total_count files changed"
    else
        echo "No files needed formatting (all files already properly formatted)"
    fi
else
    echo "❌ Prettier formatting failed!"
    exit 1
fi

# Run stylelint --fix for CSS files
echo ""
echo "Running stylelint --fix for CSS files..."
stylelint_output=$(docker run --rm -v .:/workspace -w /workspace node:lts-alpine sh -c "npm install -g stylelint stylelint-config-standard && stylelint '**/*.css' --ignore-pattern 'node_modules/**' --ignore-pattern '.svelte-kit/**' --ignore-pattern 'build/**' --fix 2>&1" || true)

if [ $? -eq 0 ]; then
    echo "✅ Stylelint completed successfully!"
else
    echo "⚠️  Stylelint reported issues (some may have been auto-fixed)"
fi

# Show stylelint output (filtered)
echo "$stylelint_output" | grep -v 'npm notice' | grep -v 'added' | grep -v 'packages' | grep -v 'up to date' | grep -v 'found' || true
