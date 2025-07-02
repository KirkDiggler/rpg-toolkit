#!/bin/bash

# Script to add GPL license headers to Go files

LICENSE_HEADER="// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

"

# Function to add header if not present
add_header() {
    file="$1"
    # Check if file already has copyright
    if ! grep -q "Copyright (C)" "$file"; then
        # Create temp file with header + original content
        echo "$LICENSE_HEADER" > tmp_header
        cat "$file" >> tmp_header
        mv tmp_header "$file"
        echo "Added header to: $file"
    else
        echo "Header already present in: $file"
    fi
}

# Find all Go files and add headers
find . -name "*.go" -type f | while read -r file; do
    add_header "$file"
done

echo "License headers added!"