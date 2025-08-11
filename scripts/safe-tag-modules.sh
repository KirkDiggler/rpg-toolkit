#!/bin/bash

# Safe module tagging script that handles existing tags gracefully

set -e

echo "🔍 Analyzing modules for tagging..."
echo ""

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to determine version bump from commit messages
determine_bump() {
  local module=$1
  local last_tag=$2
  
  if [[ -z "$last_tag" ]]; then
    compare_from="HEAD~10"  # Look at last 10 commits for new modules
  else
    compare_from="$last_tag"
  fi
  
  # Check commit messages for this module
  commits=$(git log --format="%s" "$compare_from..HEAD" -- "$module" 2>/dev/null || echo "")
  
  # Default to patch
  bump="patch"
  
  # Check for breaking changes (major)
  if echo "$commits" | grep -q "BREAKING CHANGE\|!:"; then
    bump="major"
  # Check for features (minor)
  elif echo "$commits" | grep -q "^feat"; then
    bump="minor"
  fi
  
  echo "$bump"
}

# Function to increment version
increment_version() {
  local version=$1
  local bump=$2
  
  # Remove 'v' prefix
  version=${version#v}
  
  # Split into major.minor.patch
  IFS='.' read -r major minor patch <<< "$version"
  
  case $bump in
    major)
      major=$((major + 1))
      minor=0
      patch=0
      ;;
    minor)
      minor=$((minor + 1))
      patch=0
      ;;
    patch)
      patch=$((patch + 1))
      ;;
  esac
  
  echo "v${major}.${minor}.${patch}"
}

# Track what we tag for the summary
TAGGED_MODULES=()
SKIPPED_MODULES=()
FAILED_MODULES=()

# Process each module
for modfile in $(find . -name "go.mod" -type f -not -path "./vendor/*" -not -path "./examples/*"); do
  module_path=$(dirname "$modfile" | sed 's|^\./||')
  
  # Skip root module
  if [[ "$module_path" == "." ]]; then
    continue
  fi
  
  echo -e "${BLUE}📦 Checking module: ${module_path}${NC}"
  
  # Get last tag for this module
  last_tag=$(git tag -l "${module_path}/v*" 2>/dev/null | sort -V | tail -1)
  
  # Check if module has changes since last tag
  if [[ -z "$last_tag" ]]; then
    has_changes=true  # New module
    echo "  → New module (no existing tags)"
  else
    changes=$(git diff --name-only "$last_tag" HEAD -- "$module_path" 2>/dev/null | head -1)
    has_changes=$([[ -n "$changes" ]] && echo true || echo false)
    if [[ "$has_changes" == "true" ]]; then
      echo "  → Has changes since ${last_tag}"
    else
      echo "  → No changes since ${last_tag}"
    fi
  fi
  
  if [[ "$has_changes" == "true" ]]; then
    # Determine version bump type
    bump=$(determine_bump "$module_path" "$last_tag")
    echo "  → Detected bump type: ${bump}"
    
    # Calculate new version
    if [[ -z "$last_tag" ]]; then
      new_version="v0.1.0"
    else
      current_version=${last_tag#${module_path}/}
      new_version=$(increment_version "$current_version" "$bump")
    fi
    
    # Create tag
    tag_name="${module_path}/${new_version}"
    
    # Check if tag already exists (locally or remotely)
    if git tag -l "$tag_name" | grep -q "^${tag_name}$"; then
      echo -e "  ${YELLOW}⚠️  Tag ${tag_name} already exists locally, skipping${NC}"
      SKIPPED_MODULES+=("${module_path}:${new_version}:exists_locally")
      continue
    fi
    
    # Check if tag exists on remote
    if git ls-remote --tags origin | grep -q "refs/tags/${tag_name}$"; then
      echo -e "  ${YELLOW}⚠️  Tag ${tag_name} already exists on remote, skipping${NC}"
      SKIPPED_MODULES+=("${module_path}:${new_version}:exists_remotely")
      continue
    fi
    
    # Generate release notes
    release_notes="## ${module_path} ${new_version}

"
    
    if [[ -n "$last_tag" ]]; then
      # Include commit summary
      release_notes="${release_notes}### Changes
"
      while IFS= read -r line; do
        release_notes="${release_notes}${line}
"
      done < <(git log --format="- %s (%h)" "$last_tag..HEAD" -- "$module_path" 2>/dev/null || echo "- No commit history available")
    else
      release_notes="${release_notes}Initial release
"
    fi
    
    # Create annotated tag
    if echo -e "$release_notes" | git tag -a "$tag_name" -F - 2>/dev/null; then
      echo -e "  ${GREEN}✓ Created tag: ${tag_name}${NC}"
      TAGGED_MODULES+=("${module_path}:${new_version}")
    else
      echo -e "  ${RED}✗ Failed to create tag: ${tag_name}${NC}"
      FAILED_MODULES+=("${module_path}:${new_version}")
    fi
  fi
  echo ""
done

# Summary
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "📊 Summary:"
echo ""

if [[ ${#TAGGED_MODULES[@]} -gt 0 ]]; then
  echo -e "${GREEN}✅ Successfully tagged ${#TAGGED_MODULES[@]} module(s):${NC}"
  for item in "${TAGGED_MODULES[@]}"; do
    IFS=':' read -r module version <<< "$item"
    echo "   • ${module} → ${version}"
  done
  echo ""
fi

if [[ ${#SKIPPED_MODULES[@]} -gt 0 ]]; then
  echo -e "${YELLOW}⚠️  Skipped ${#SKIPPED_MODULES[@]} module(s) (tags already exist):${NC}"
  for item in "${SKIPPED_MODULES[@]}"; do
    IFS=':' read -r module version reason <<< "$item"
    echo "   • ${module} → ${version} (${reason})"
  done
  echo ""
fi

if [[ ${#FAILED_MODULES[@]} -gt 0 ]]; then
  echo -e "${RED}✗ Failed to tag ${#FAILED_MODULES[@]} module(s):${NC}"
  for item in "${FAILED_MODULES[@]}"; do
    IFS=':' read -r module version <<< "$item"
    echo "   • ${module} → ${version}"
  done
  echo ""
fi

# Push tags if any were created
if [[ ${#TAGGED_MODULES[@]} -gt 0 ]]; then
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo ""
  echo "📤 Ready to push tags to remote"
  echo ""
  echo "To push the new tags, run:"
  echo -e "${BLUE}  git push origin --tags${NC}"
  echo ""
  echo "Or to push specific tags only:"
  for item in "${TAGGED_MODULES[@]}"; do
    IFS=':' read -r module version <<< "$item"
    echo -e "${BLUE}  git push origin ${module}/${version}${NC}"
  done
else
  if [[ ${#SKIPPED_MODULES[@]} -eq 0 ]]; then
    echo "No modules needed tagging."
  fi
fi