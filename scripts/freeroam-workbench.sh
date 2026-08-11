#!/bin/bash

# Launch the free-roam encounter workbench (the pre-UI loop) from any
# checkout of this repo — resolves its own location, so it works from a
# worktree today and the main clone after merge.

set -e

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT/rulebooks/dnd5e/encounter"
exec go run ./cmd/freeroam-workbench "$@"
