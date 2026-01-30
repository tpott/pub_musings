#!/bin/bash
# Lint frontend source files for maximum line count.
#
# - Error threshold: 4000 lines (fails the lint)
# - Warning threshold: 1000 lines (advisory, does not fail)
# - Test files (*test.ts) are excluded.
#
# Thresholds are set above current largest files to cap future growth.
# upload.astro is 3844 lines — the error limit prevents further growth.
# Files above the warning threshold are candidates for splitting.

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
FRONTEND_SRC="$PROJECT_ROOT/frontend/src"

ERROR_LIMIT=4000
WARN_LIMIT=1000

has_error=0
has_warning=0

while IFS= read -r file; do
    # Skip test files
    if [[ "$file" == *.test.ts ]]; then
        continue
    fi

    line_count=$(wc -l < "$file")

    # Trim whitespace from wc output
    line_count=$(echo "$line_count" | tr -d ' ')

    rel_path="${file#$PROJECT_ROOT/}"

    if [ "$line_count" -gt "$ERROR_LIMIT" ]; then
        echo "ERROR: $rel_path has $line_count lines (max $ERROR_LIMIT)"
        has_error=1
    elif [ "$line_count" -gt "$WARN_LIMIT" ]; then
        echo "WARNING: $rel_path has $line_count lines (recommended max $WARN_LIMIT)"
        has_warning=1
    fi
done < <(find "$FRONTEND_SRC" \( -name '*.astro' -o -name '*.ts' \) -type f | sort)

if [ "$has_error" -eq 1 ]; then
    echo ""
    echo "Frontend file size lint FAILED — files exceed $ERROR_LIMIT lines."
    echo "Split large files before adding more code."
    exit 1
fi

if [ "$has_warning" -eq 1 ]; then
    echo ""
    echo "Frontend file size lint passed with warnings."
    echo "Consider splitting files above $WARN_LIMIT lines."
else
    echo "Frontend file size lint passed — all files within limits."
fi
