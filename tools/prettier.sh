#!/bin/sh

# Format Markdown and TypeScript files with Prettier for PVMSS project
echo "Formatting Markdown and TypeScript files with Prettier..."

# Check if any files exist (excluding node_modules, build, .svelte-kit)
md_count=$(find . -name '*.md' -type f -not -path '*/node_modules/*' -not -path '*/.svelte-kit/*' -not -path '*/build/*' 2>/dev/null | wc -l)
ts_count=$(find . -name '*.ts' -type f -not -path '*/node_modules/*' -not -path '*/.svelte-kit/*' -not -path '*/build/*' 2>/dev/null | wc -l)
total_count=$((md_count + ts_count))

if [ "$total_count" -eq 0 ]; then
    echo "⚠️  No Markdown or TypeScript files found in project"
    exit 0
fi

echo "✅ Found files:"
echo "  Markdown: $md_count files"
echo "  TypeScript: $ts_count files"
echo "  Total: $total_count files"

# Run Prettier and capture output
echo "Running Prettier to format files..."
output=$(docker run --rm -v .:/workspace -w /workspace node:lts-alpine sh -c "npm install -g prettier && prettier --write '**/*.{md,ts,tsx}' --ignore 'node_modules/**' --ignore '.svelte-kit/**' --ignore 'build/**' 2>&1" || true)

if [ $? -eq 0 ]; then
    echo "✅ Prettier formatting completed successfully!"
    
    # Show only changed files (filter out unchanged and installation noise)
    changed_files=$(echo "$output" | grep -E '\.(md|ts|tsx)' | grep -v 'unchanged' | grep -v 'node_modules' | grep -v '.svelte-kit' | grep -v 'build')
    
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
