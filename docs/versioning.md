# Module Versioning Guide

This guide explains how versioning works in the rpg-toolkit multi-module repository.

## Overview

The rpg-toolkit uses Go's multi-module repository pattern where each module is independently versioned using git tags with the format `module/path/vX.Y.Z`.

## Current Module Versions

Run `make check-versions` to see the current version of all modules and whether they have uncommitted changes.

## Versioning Strategy

### Semantic Versioning

We follow [Semantic Versioning](https://semver.org/):
- **MAJOR** (X.0.0): Breaking API changes
- **MINOR** (0.X.0): New features, backwards compatible
- **PATCH** (0.0.X): Bug fixes, backwards compatible

### Version Determination

When using conventional commits:
- `feat:` commits trigger a **minor** version bump
- `fix:` commits trigger a **patch** version bump
- Commits with `BREAKING CHANGE` or `!:` trigger a **major** version bump
- Other commits (`chore:`, `docs:`, `style:`, `refactor:`, `test:`) trigger **patch** bumps

## Automated Versioning (CI)

### Auto-tagging on Main

The `auto-tag-modules.yml` workflow runs on every push to main and:
1. Detects which modules have changes
2. Analyzes commit messages to determine version bump type
3. Creates and pushes appropriate tags
4. Generates release notes

### Manual Release Workflow

Use the `release-module.yml` workflow from GitHub Actions UI to:
1. Manually release a specific module
2. Specify exact version number
3. Add custom release notes
4. Create GitHub Release

### Batch Tagging

Use the `tag-modules.yml` workflow to:
1. Tag multiple modules at once
2. Specify bump type (patch/minor/major)
3. Auto-detect changed modules or specify manually

## Local Versioning

### Check Current Versions

```bash
# See all module versions
make check-versions

# Or use the script directly
./scripts/check-versions.sh
```

### Tag a Module Locally

```bash
# Tag a specific module
make tag-module MODULE=tools/spatial VERSION=v0.2.0

# Then push the tag
git push origin tools/spatial/v0.2.0
```

### Release a Module (with tests)

```bash
# Test and tag a module
make release-module MODULE=tools/spatial VERSION=v0.2.0

# Then push the tag
git push origin tools/spatial/v0.2.0
```

## Using Versioned Modules

### In Your Project

```bash
# Install a specific version
go get github.com/KirkDiggler/rpg-toolkit/tools/spatial@tools/spatial/v0.2.0

# Or use the latest version
go get github.com/KirkDiggler/rpg-toolkit/tools/spatial@latest

# Update to latest
go get -u github.com/KirkDiggler/rpg-toolkit/tools/spatial
```

### In go.mod

```go
require (
    github.com/KirkDiggler/rpg-toolkit/core v0.1.0
    github.com/KirkDiggler/rpg-toolkit/tools/spatial v0.2.0
)
```

## Module Dependencies

When one rpg-toolkit module depends on another:

1. **During Development**: Go creates pseudo-versions automatically
2. **After Release**: Update to use tagged versions

Example workflow:
```bash
# Working on tools/spatial which depends on core
cd tools/spatial

# Update to latest core version
go get -u github.com/KirkDiggler/rpg-toolkit/core

# This pulls the latest tagged version or creates a pseudo-version
```

## Best Practices

### Commit Messages

Use conventional commits for automatic version detection:
```bash
# Minor version bump (new feature)
git commit -m "feat(spatial): add pathfinding algorithm"

# Patch version bump (bug fix)
git commit -m "fix(spatial): correct boundary check in hex grid"

# Major version bump (breaking change)
git commit -m "feat(spatial)!: redesign Grid interface

BREAKING CHANGE: Grid.GetNeighbors now returns error as second value"
```

### When to Release

- **Immediately**: Bug fixes that affect users
- **Batched**: New features can be accumulated
- **Coordinated**: Breaking changes across multiple modules

### Pre-release Testing

Before creating a release:
1. Run `make pre-commit` locally
2. Ensure CI passes on your PR
3. Test dependent modules if making breaking changes

## Troubleshooting

### Module Not Found

If `go get` can't find a module version:
1. Check the tag exists: `git tag -l "module/path/*"`
2. Ensure tag is pushed: `git push origin module/path/vX.Y.Z`
3. Wait a moment for the Go proxy to update

### Version Conflicts

If you get version conflicts:
1. Run `go mod tidy` in the affected module
2. Check for circular dependencies
3. Ensure all modules use compatible versions

### CI Failures

If the auto-tag workflow fails:
1. Check commit message format
2. Ensure no merge conflicts
3. Verify module has go.mod file

## Module Tag History

To see the version history of a module:
```bash
# List all tags for a module
git tag -l "tools/spatial/v*" | sort -V

# See what changed in each version
git log --oneline tools/spatial/v0.1.0..tools/spatial/v0.2.0 -- tools/spatial
```

## Questions?

For issues or questions about versioning:
1. Check existing module tags as examples
2. Review the CI workflow files in `.github/workflows/`
3. Ask in the project discussions