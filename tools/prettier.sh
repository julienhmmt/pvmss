#!/bin/sh

# Format Markdown files with Prettier for PVMSS project
echo "Formatting Markdown files with Prettier..."

# Check if any Markdown files exist (excluding node_modules)
if [ -z "$(find . -name '*.md' -type f -not -path '*/node_modules/*' 2>/dev/null)" ]; then
    echo "⚠️  No Markdown files found in project"
    exit 0
fi

echo "✅ Found Markdown files:"
find . -name '*.md' -type f -not -path '*/node_modules/*' | head -10
total_count=$(find . -name '*.md' -type f -not -path '*/node_modules/*' | wc -l)
if [ "$total_count" -gt 10 ]; then
    echo "... and $(( total_count - 10 )) more files"
fi
echo "Total: $total_count files"

# Run Prettier and capture output
echo "Running Prettier to format Markdown files..."
output=$(docker run --rm -v .:/workspace -w /workspace node:lts-alpine sh -c "npm install -g prettier && prettier --write '**/*.md' --ignore 'node_modules/**' 2>&1" || true)

if [ $? -eq 0 ]; then
    echo "✅ Prettier formatting completed successfully!"
    
    # Show only changed files (filter out unchanged and installation noise)
    changed_files=$(echo "$output" | grep '\.md' | grep -v 'unchanged' | grep -v 'node_modules')
    
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
