#!/bin/bash

# Script to check current versions of all modules in rpg-toolkit

set -e

echo "🔍 Checking module versions..."
echo ""

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Find all modules
modules=()
while IFS= read -r modfile; do
    module_path=$(dirname "$modfile" | sed 's|^\./||')
    # Skip root module
    if [[ "$module_path" != "." ]]; then
        modules+=("$module_path")
    fi
done < <(find . -name "go.mod" -type f -not -path "./vendor/*" -not -path "./examples/*")

# Sort modules
IFS=$'\n' sorted_modules=($(sort <<<"${modules[*]}"))
unset IFS

echo "Found ${#sorted_modules[@]} modules:"
echo ""

# Check each module
for module in "${sorted_modules[@]}"; do
    # Get latest tag for this module
    latest_tag=$(git tag -l "${module}/v*" 2>/dev/null | sort -V | tail -1)
    
    if [[ -n "$latest_tag" ]]; then
        version=${latest_tag#${module}/}
        
        # Check if module has changes since last tag
        changes=$(git diff --name-only "$latest_tag" HEAD -- "$module" 2>/dev/null | head -1)
        
        if [[ -n "$changes" ]]; then
            echo -e "${YELLOW}📦 ${module}${NC}"
            echo -e "   Version: ${version} ${YELLOW}(has uncommitted changes)${NC}"
        else
            echo -e "${GREEN}📦 ${module}${NC}"
            echo -e "   Version: ${version}"
        fi
        
        # Show how to import
        echo -e "   Import:  go get github.com/KirkDiggler/rpg-toolkit/${module}@${latest_tag}"
    else
        echo -e "${RED}📦 ${module}${NC}"
        echo -e "   Version: ${RED}Not yet released${NC}"
        echo -e "   Import:  Will be available after first release"
    fi
    echo ""
done

# Summary
echo "---"
echo ""
tagged_count=$(git tag -l "*/v*" | cut -d'/' -f1 | sort -u | wc -l)
echo "📊 Summary:"
echo "   Total modules: ${#sorted_modules[@]}"
echo "   Tagged modules: ${tagged_count}"
echo "   Untagged modules: $((${#sorted_modules[@]} - tagged_count))"

# Check for modules with changes
modules_with_changes=0
for module in "${sorted_modules[@]}"; do
    latest_tag=$(git tag -l "${module}/v*" 2>/dev/null | sort -V | tail -1)
    if [[ -n "$latest_tag" ]]; then
        changes=$(git diff --name-only "$latest_tag" HEAD -- "$module" 2>/dev/null | head -1)
        if [[ -n "$changes" ]]; then
            ((modules_with_changes++))
        fi
    fi
done

if [[ $modules_with_changes -gt 0 ]]; then
    echo ""
    echo -e "${YELLOW}⚠️  ${modules_with_changes} module(s) have changes since their last release${NC}"
    echo "   Run 'make release' or use GitHub Actions to create new versions"
fi