#!/bin/bash

# Update copyright year in source files
# Usage: ./update_copyright [year]
# If no year is provided, uses current year

set -e

# Determine which sed to use
SED="sed"
if [[ "$OSTYPE" == "darwin"* ]]; then
    if command -v gsed >/dev/null 2>&1; then
        SED="gsed"
    else
        echo "Error: GNU sed (gsed) not found on macOS."
        echo "Install it with: brew install gnu-sed"
        exit 1
    fi
fi

# Get the target year (current year if not provided)
if [ $# -eq 0 ]; then
    YEAR=$(date +%Y)
else
    YEAR=$1
fi

# Generate copyright string based on year
if [ $YEAR -eq 2025 ]; then
    COPYRIGHT_TEXT="Copyright 2025 Poiesic Systems"
else
    COPYRIGHT_TEXT="Copyright 2025 - $YEAR Poiesic Systems"
fi

echo "Updating copyright to: $COPYRIGHT_TEXT"

# Find all source files with copyright headers
find . -name "*.go" -type f | while read -r file; do
    if grep -q "Copyright [0-9]\{4\}\( - [0-9]\{4\}\)\? Poiesic Systems" "$file"; then
        echo "Updating $file"
        # Use sed to replace the copyright year/range
        $SED -i "s/Copyright [0-9]\{4\}\( - [0-9]\{4\}\)\? Poiesic Systems/$COPYRIGHT_TEXT/" "$file"
    fi
done

echo "Copyright updated in all applicable files."