# Retire the Top-Level Legacy Encounter Module Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Delete the unconsumed top-level `rpg-toolkit/encounter` module and make current toolkit documentation and verification describe only the active session stack.

**Architecture:** Perform a hard source deletion with no compatibility layer. Preserve historical documentation behind explicit retirement markers, leave `rulebooks/dnd5e/{encounter,resolution,session}` untouched, and prove every remaining module through the repository's dynamic full checks.

**Tech Stack:** Go 1.25 multi-module repository, Bash verification, Markdown documentation, GitHub Actions changed-module/full-check workflows.

**Spec:** `docs/superpowers/specs/2026-08-23-retire-legacy-encounter-design.md`

## Global Constraints

- Implement only [rpg-toolkit#1215](https://github.com/KirkDiggler/rpg-toolkit/issues/1215) in this repository.
- Delete the entire top-level `encounter/` module; do not move its source, add a shim, or mint a compatibility tag.
- Do not modify `rulebooks/dnd5e/encounter`, `rulebooks/dnd5e/resolution`, or `rulebooks/dnd5e/session`.
- Do not implement ADR-0045 or any part of #1198.
- Preserve historical ADR, journey, and plan text unless it currently claims the retired module remains supported.
- Treat rpg-api, rpg-deployment, and rpg-project references as separately owned follow-up findings, not files for this branch.
- Stop if a current Go consumer of `github.com/KirkDiggler/rpg-toolkit/encounter` appears.
- Use the existing branch `feat/1215-retire-legacy-encounter`; merge `origin/main` into it if needed and never rebase it.
- Never commit a local `replace`, `go.work`, generated artifact, coverage output, or unrelated module change.
- Put exactly one closing keyword in the PR body: `Closes #1215`.

---

## File Structure

| Path | Change | Responsibility |
|---|---|---|
| `encounter/` | Delete | Remove the complete legacy module, including source, module metadata, tests, fixtures, and workbench commands. |
| `README.md` | Modify | Describe the active D&D 5e module stack and stop advertising the legacy module or a hand-maintained module count. |
| `docs/architecture/components/encounter.md` | Modify | Retain a stable historical link target with an unmistakable retirement tombstone. |
| `docs/quality.md` | Modify | Remove the deleted module from current grading and preserve only a concise retirement record. |
| `docs/status.md` | Modify | State the current session direction, mark old encounter sections historical, and remove the module from active confidence inventory. |
| `docs/adr/0034-where-encounter-logic-lives.md` | Modify | Mark the unexecuted split plan superseded while preserving its rationale. |
| `docs/adr/DECISIONS.md` | Modify | Correct the compressed decision so readers do not believe the planned split shipped. |

No new production source, compatibility package, test harness, or module-census script is created.

---

### Task 1: Prove Consumer Absence and Delete the Legacy Module

**Files:**
- Delete: `encounter/**`
- Test: structural shell assertions and ordered-workspace Go consumer scans

**Interfaces:**
- Consumes: current ordered workspace checkouts and the exact legacy module path `github.com/KirkDiggler/rpg-toolkit/encounter`.
- Produces: a toolkit tree with no top-level `encounter/` directory and 23 remaining dynamically discovered Go module roots.

- [ ] **Step 1: Read the approved spec and confirm branch provenance**

Run:

```bash
cd /home/kirk/game-dev/rpg-toolkit/.worktrees/1215-retire-legacy-encounter
test "$(git branch --show-current)" = "feat/1215-retire-legacy-encounter"
git fetch origin
if ! git merge-base --is-ancestor origin/main HEAD; then
  git merge --no-edit origin/main
fi
grep -Fq '**Status:** Approved in session on 2026-08-23' \
  docs/superpowers/specs/2026-08-23-retire-legacy-encounter-design.md
git status --short
```

Expected: all commands exit `0`; status is clean before implementation; `HEAD` contains current `origin/main` plus the approved spec. Any integration is a merge, never a rebase.

- [ ] **Step 2: Capture the external-consumer baseline**

Run:

```bash
workspace=/home/kirk/game-dev
legacy='github\.com/KirkDiggler/rpg-toolkit/encounter'

for repo in rpg-project rpg-api rpg-api-protos rpg-dnd5e-web rpg-game-assets rpg-deployment; do
  if rg -n --hidden \
    --glob '!**/.git/**' \
    --glob '!.worktrees/**' \
    --glob '!.pi-worktrees/**' \
    --glob '!**/vendor/**' \
    --glob '*.go' \
    '"github\.com/KirkDiggler/rpg-toolkit/encounter(?:/[^"[:space:]]*)?"' \
    "$workspace/$repo"; then
    echo "error: current Go source consumer found in $repo" >&2
    exit 1
  fi
  if rg -n --hidden \
    --glob '!**/.git/**' \
    --glob '!.worktrees/**' \
    --glob '!.pi-worktrees/**' \
    --glob '!**/vendor/**' \
    --glob 'go.mod' \
    'github\.com/KirkDiggler/rpg-toolkit/encounter([[:space:]]|$)' \
    "$workspace/$repo"; then
    echo "error: current go.mod consumer found in $repo" >&2
    exit 1
  fi
done

(
  cd "$workspace/rpg-api"
  go mod why -m github.com/KirkDiggler/rpg-toolkit/encounter
  ! go mod graph | grep -F 'github.com/KirkDiggler/rpg-toolkit/encounter'
)
```

Expected: no source/module scan prints a consumer; `go mod why` prints `(main module does not need module github.com/KirkDiggler/rpg-toolkit/encounter)`; the graph assertion exits `0`.

- [ ] **Step 3: Run the retirement structural assertion and verify it is red**

Run:

```bash
cd /home/kirk/game-dev/rpg-toolkit/.worktrees/1215-retire-legacy-encounter
test ! -e encounter
```

Expected: non-zero because `encounter/` still exists. This is the deletion slice's red test.

- [ ] **Step 4: Delete only the tracked legacy module**

Run:

```bash
cd /home/kirk/game-dev/rpg-toolkit/.worktrees/1215-retire-legacy-encounter
git rm -r -- encounter
```

Expected: Git stages deletion of 177 tracked files under `encounter/`; no path outside `encounter/` is staged.

- [ ] **Step 5: Run the structural assertion and module scan green**

Run:

```bash
test ! -e encounter
test -z "$(git ls-files encounter)"

test "$(find . \
  -path './vendor' -prune -o \
  -path './.worktrees' -prune -o \
  -name go.mod -type f -print | wc -l)" -eq 23

! rg -n --hidden \
  --glob '!**/.git/**' \
  --glob '!.worktrees/**' \
  --glob '!**/vendor/**' \
  --glob '*.go' \
  --glob 'go.mod' \
  'github\.com/KirkDiggler/rpg-toolkit/encounter([/"[:space:]]|$)' .

outside_encounter="$(git diff --cached --name-only | grep -v '^encounter/' || true)"
if [ -n "$outside_encounter" ]; then
  printf 'error: deletion task staged paths outside encounter/:\n%s\n' \
    "$outside_encounter" >&2
  exit 1
fi
```

Expected: all positive assertions pass; module count is `23`; no remaining toolkit Go source or module metadata names the retired module; staged paths are all under `encounter/`.

- [ ] **Step 6: Commit the source retirement**

Run:

```bash
git commit -m "chore!: retire the top-level encounter module (#1215)"
git status --short
```

Expected: commit succeeds; working tree is clean; the pre-commit hook reports no added/modified Go files because this commit contains deletions only. Full remaining-module verification is deliberately performed in Task 5.

---

### Task 2: Replace Current Module Guidance with a Historical Tombstone

**Files:**
- Modify: `README.md:1-49`
- Modify: `docs/architecture/components/encounter.md:1-16`
- Test: focused Markdown assertions

**Interfaces:**
- Consumes: the active module paths `rulebooks/dnd5e`, `rulebooks/dnd5e/behavior`, `rulebooks/dnd5e/encounter`, `rulebooks/dnd5e/resolution`, and `rulebooks/dnd5e/session`.
- Produces: current entry-point documentation that names only supported modules while preserving the old component page as a historical link target.

- [ ] **Step 1: Run the current-guidance assertion and verify it is red**

Run:

```bash
if rg -n \
  'currently D&D-5e-coupled encounter SDK|currently contains [0-9]+ Go module roots|Current composition.*`encounter`' \
  README.md; then
  exit 1
fi
grep -Fq '> **Retired 2026-08-23 by #1215.**' \
  docs/architecture/components/encounter.md
```

Expected: non-zero. `README.md` still advertises the legacy SDK and hard-coded count, and the component page has no retirement banner.

- [ ] **Step 2: Rewrite the README introduction and module map**

Replace the opening description with:

```markdown
RPG Toolkit is a collection of independently versioned Go modules for building
RPG rules engines and game hosts. It contains reusable foundations and tools,
the D&D 5e rulebook, and the active D&D 5e encounter composition, resolution
machines, and session host seam. The toolkit owns game rules; a host owns
storage, transport, and request orchestration.
```

Replace the hard-coded count paragraph with:

```markdown
The repository has no root `go.mod`. Each module root has its own
dependency/version boundary and module-prefixed Git tags. Repository commands
discover current module roots from their `go.mod` files rather than a
hand-maintained count. Each module has its own test command, although some
packages/modules currently contain no test files.
```

Replace the Rulebooks row and remove the legacy `Current composition` row so the table contains:

```markdown
| Rulebooks and live D&D 5e stack | `rulebooks/dnd5e`, `rulebooks/dnd5e/behavior`, `rulebooks/dnd5e/encounter`, `rulebooks/dnd5e/resolution`, `rulebooks/dnd5e/session` | D&D 5e content and rules, encounter composition, interaction resolution, monster behavior, and the host-facing session seam |
```

Do not alter the Core, Mechanics, Play primitives, or Tools rows.

- [ ] **Step 3: Convert the component page into a retirement tombstone**

Replace the page front matter and opening through the old introductory paragraph with:

```markdown
---
name: retired top-level encounter module
description: Historical record of the deleted orchestrator-facing encounter SDK
updated: 2026-08-23
confidence: high — rpg-toolkit#1215 retired the unconsumed module after rpg-api moved to the session stack
---

# Top-level encounter module (retired)

> **Retired 2026-08-23 by #1215.** The module
> `github.com/KirkDiggler/rpg-toolkit/encounter` no longer exists. Current game
> execution uses [`rulebooks/dnd5e/encounter`](../../../rulebooks/dnd5e/encounter),
> [`rulebooks/dnd5e/resolution`](../../../rulebooks/dnd5e/resolution), and
> [`rulebooks/dnd5e/session`](../../../rulebooks/dnd5e/session). The material
> below is retained as historical implementation context, not supported API
> guidance.

## Historical record

**Former path:** `encounter/`  
**Former module:** `github.com/KirkDiggler/rpg-toolkit/encounter`  
**Last grade:** B+

The retired encounter SDK was the orchestrator-facing facade for running an
encounter (combat, free-roam, social) end-to-end. Game servers loaded an
encounter from persisted state, called verb methods, serialized through
`ToData`, and saved. Player-facing events flowed through its process-scoped
broker.
```

Keep the existing `## Internal layout` section and everything after it unchanged as historical content.

- [ ] **Step 4: Run focused documentation assertions**

Run:

```bash
! rg -n \
  'currently D&D-5e-coupled encounter SDK|currently contains [0-9]+ Go module roots|Current composition.*`encounter`' \
  README.md
grep -Fq '`rulebooks/dnd5e/session`' README.md
grep -Fq '> **Retired 2026-08-23 by #1215.**' \
  docs/architecture/components/encounter.md
grep -Fq '## Historical record' docs/architecture/components/encounter.md
git diff --check
```

Expected: all assertions pass and `git diff --check` prints nothing.

- [ ] **Step 5: Commit current guidance and the tombstone**

Run:

```bash
git add README.md docs/architecture/components/encounter.md
git commit -m "docs: mark the top-level encounter module retired"
git status --short
```

Expected: commit succeeds and working tree is clean.

---

### Task 3: Remove the Retired Module from Active Quality and Status Inventories

**Files:**
- Modify: `docs/quality.md:1-108,420-448`
- Modify: `docs/status.md:1-16,633-646,661-690`
- Test: focused active-inventory assertions

**Interfaces:**
- Consumes: the retirement tombstone from Task 2.
- Produces: living quality/status documents that no longer grade the deleted module or present its gaps as active work.

- [ ] **Step 1: Run active-inventory assertions and verify they are red**

Run:

```bash
! rg -n \
  '^### encounter — B\+|^\| B\+ \|.*(^|, )encounter(,|$)|^\| encounter \|' \
  docs/quality.md docs/status.md
grep -Fq '## Current direction' docs/status.md
grep -Fq '### Retired: top-level encounter module' docs/quality.md
```

Expected: non-zero because quality still grades `encounter`, status still contains an active confidence row, and neither retirement section exists.

- [ ] **Step 2: Replace the active encounter quality grade with a concise retired record**

Update `docs/quality.md` front matter to `updated: 2026-08-23` and append this sentence to its confidence value:

```text
The top-level encounter module's former B+ assessment is retained only as a retirement note after #1215.
```

Replace the complete section from `### encounter — B+` up to, but not including, `### events — B+` with:

```markdown
### Retired: top-level encounter module

The former `github.com/KirkDiggler/rpg-toolkit/encounter` module was graded B+
before retirement. rpg-toolkit#1215 deleted it after rpg-api moved to the active
session stack. Its detailed assessment remains in the
[historical component record](architecture/components/encounter.md); it is not
part of the current module scorecard.

```

In the grade distribution:

- change the heading date to `2026-08-23`;
- remove `encounter` from the B+ row;
- remove the sentence claiming Wave 2.11d moves `encounter` from B to B+;
- keep the combat and conditions grading explanation intact.

- [ ] **Step 3: Mark old status material historical and remove active confidence**

Update `docs/status.md` front matter to `updated: 2026-08-23` and prepend its confidence value with:

```text
high for the 2026-08-23 retirement state; older delivery entries are retained as dated history;
```

After the living-doc paragraph, insert:

```markdown
## Current direction

The top-level `github.com/KirkDiggler/rpg-toolkit/encounter` module was retired
by rpg-toolkit#1215 after rpg-api moved to `rulebooks/dnd5e/session`. Current
game execution lives in the independently versioned D&D 5e encounter
composition, resolution, and session modules. Older sections below retain the
legacy module's delivery history and must not be read as current support.

```

Make these two focused historical labels:

- rename `## Active work` to `## Historical top-level encounter delivery record`;
- rename `### Encounter SDK` under Known rough edges to `### Retired top-level Encounter SDK (historical)` and add one sentence that its listed gaps are preserved history, not active module debt.

Delete the `| encounter | High ... |` row from Per-subsystem confidence. Keep all dated delivery prose and all other subsystem rows unchanged.

- [ ] **Step 4: Run the active-inventory assertions green**

Run:

```bash
! rg -n \
  '^### encounter — B\+|^\| B\+ \|.*(^|, )encounter(,|$)|^\| encounter \|' \
  docs/quality.md docs/status.md
grep -Fq '## Current direction' docs/status.md
grep -Fq '## Historical top-level encounter delivery record' docs/status.md
grep -Fq '### Retired top-level Encounter SDK (historical)' docs/status.md
grep -Fq '### Retired: top-level encounter module' docs/quality.md
git diff --check
```

Expected: all assertions pass; historical narrative remains; no whitespace errors appear.

- [ ] **Step 5: Commit living status and quality reconciliation**

Run:

```bash
git add docs/quality.md docs/status.md
git commit -m "docs: remove retired encounter from active inventories"
git status --short
```

Expected: commit succeeds and working tree is clean.

---

### Task 4: Mark ADR-0034's Planned Split Superseded

**Files:**
- Modify: `docs/adr/0034-where-encounter-logic-lives.md:1-14`
- Modify: `docs/adr/DECISIONS.md:31-33`
- Test: `scripts/check-decisions.sh` and focused wording assertions

**Interfaces:**
- Consumes: the historical decision that proposed splitting the top-level module.
- Produces: an ADR corpus and digest that agree the split was never executed and the legacy module was instead replaced by the separately built session stack and deleted.

- [ ] **Step 1: Run the supersession assertion and verify it is red**

Run:

```bash
grep -Fq '**Superseded in implementation (2026-08-23).**' \
  docs/adr/0034-where-encounter-logic-lives.md
```

Expected: non-zero because ADR-0034 still says its split is accepted for later execution.

- [ ] **Step 2: Replace ADR-0034's status section**

Replace the existing `## Status` body, stopping before `## Context`, with:

```markdown
## Status

**Superseded in implementation (2026-08-23).** The proposed split was never
performed. The active `rulebooks/dnd5e/encounter`, resolution, and session stack
was built separately; rpg-api moved to that stack in PRs #801/#804; and
rpg-toolkit#1215 deleted the now-unconsumed top-level module. The original
reasoning and alternatives remain below as history.

The general rule survives: a package that mixes reusable infrastructure with
one game's rules has a false boundary. The superseding implementation chose a
clean new composition and host seam rather than relocating the old module's two
halves.

```

Do not rewrite the Context, inventory, options, or consequences sections.

- [ ] **Step 3: Correct the ADR digest entry**

Replace the current ADR-0034 digest entry with:

```markdown
- **0034 (superseded)** — Proposed splitting the old top-level encounter along
  its generic/rulebook seam; that move never shipped. The session stack was
  built separately and #1215 retired the old module. *Standing rule: when one
  module is "generic" plus "one game's rules", its boundary is false.*
```

- [ ] **Step 4: Run ADR checks**

Run:

```bash
grep -Fq '**Superseded in implementation (2026-08-23).**' \
  docs/adr/0034-where-encounter-logic-lives.md
grep -Fq '**0034 (superseded)**' docs/adr/DECISIONS.md
! grep -Fq '**0034** — The encounter was split' docs/adr/DECISIONS.md
./scripts/check-decisions.sh
git diff --check
```

Expected: all commands exit `0`; `check-decisions.sh` reports that every ADR has a digest entry.

- [ ] **Step 5: Commit the supersession record**

Run:

```bash
git add docs/adr/0034-where-encounter-logic-lives.md docs/adr/DECISIONS.md
git commit -m "docs: supersede the legacy encounter split decision"
git status --short
```

Expected: commit succeeds and working tree is clean.

---

### Task 5: Run Full Verification and Deliver the Retirement PR

**Files:**
- Modify: no repository files unless a verification command exposes a task-scoped defect
- GitHub: comment on issue #1215, push the branch, open the PR, apply `full-check`
- Test: complete remaining-module and consumer verification

**Interfaces:**
- Consumes: Tasks 1-4 and current `origin/main`.
- Produces: one reviewable rpg-toolkit PR that closes #1215 and unblocks #1198, plus a durable issue comment recording separately owned workspace findings.

- [ ] **Step 1: Reconcile current main without rebasing**

Run:

```bash
git fetch origin
if ! git merge-base --is-ancestor origin/main HEAD; then
  git merge --no-edit origin/main
fi
git status --short --branch
```

Expected: branch contains current `origin/main`; any integration uses a merge commit, never rebase; working tree is clean. If a conflict touches the open ADR-0045 documentation from PR #1214, preserve both ADR-0045's action decision and this branch's ADR-0034 retirement correction.

- [ ] **Step 2: Re-run deletion and consumer proofs against the final tree**

Run:

```bash
test ! -e encounter
test -z "$(git ls-files encounter)"

workspace=/home/kirk/game-dev
for repo in rpg-project rpg-api rpg-api-protos rpg-dnd5e-web rpg-game-assets rpg-deployment; do
  ! rg -n --hidden \
    --glob '!**/.git/**' \
    --glob '!.worktrees/**' \
    --glob '!.pi-worktrees/**' \
    --glob '!**/vendor/**' \
    --glob '*.go' \
    '"github\.com/KirkDiggler/rpg-toolkit/encounter(?:/[^"[:space:]]*)?"' \
    "$workspace/$repo"
  ! rg -n --hidden \
    --glob '!**/.git/**' \
    --glob '!.worktrees/**' \
    --glob '!.pi-worktrees/**' \
    --glob '!**/vendor/**' \
    --glob 'go.mod' \
    'github\.com/KirkDiggler/rpg-toolkit/encounter([[:space:]]|$)' \
    "$workspace/$repo"
done

(
  cd "$workspace/rpg-api"
  go mod why -m github.com/KirkDiggler/rpg-toolkit/encounter
  ! go mod graph | grep -F 'github.com/KirkDiggler/rpg-toolkit/encounter'
)
```

Expected: all assertions pass and rpg-api still reports the legacy module is not needed.

- [ ] **Step 3: Run every remaining toolkit gate**

Run, in this order:

```bash
make pre-commit
make test-all
make lint-all
./scripts/check-decisions.sh
./scripts/check-versions.sh | tee /tmp/rpg-toolkit-1215-versions.txt
grep -Fq '   Total modules: 23' /tmp/rpg-toolkit-1215-versions.txt
! grep -Fq 'go get github.com/KirkDiggler/rpg-toolkit/encounter@' \
  /tmp/rpg-toolkit-1215-versions.txt
git diff --check origin/main...HEAD
git status --short --branch
```

Expected:

- every command exits `0`;
- test/lint/tidy discovery covers 23 remaining module roots;
- version census does not list top-level `encounter` (entries such as `rulebooks/dnd5e/encounter` remain valid);
- no command creates an uncommitted change.

If a gate fails, stop and diagnose it. Do not weaken the gate, skip a module, or claim the branch ready from a baseline comparison alone.

- [ ] **Step 4: Review the complete diff and commit graph**

Run:

```bash
git diff --stat origin/main...HEAD
git diff --name-status origin/main...HEAD
git log --oneline --decorate origin/main..HEAD
git diff origin/main...HEAD -- \
  README.md \
  docs/architecture/components/encounter.md \
  docs/quality.md \
  docs/status.md \
  docs/adr/0034-where-encounter-logic-lives.md \
  docs/adr/DECISIONS.md
```

Expected: source changes are the complete `encounter/` deletion; surviving file changes are the approved spec, this plan, and the six scoped current/historical documentation files; no active D&D 5e module is modified.

- [ ] **Step 5: Record separately owned workspace findings on #1215**

Run:

```bash
gh issue comment 1215 --repo KirkDiggler/rpg-toolkit --body-file - <<'EOF'
Consumer verification confirmed that no current ordered-workspace Go source or `go.mod` imports `github.com/KirkDiggler/rpg-toolkit/encounter`; current rpg-api also reports the module is not needed and has no module-graph edge to it.

Separately owned contributor-tooling references remain and are not Go consumers:

- rpg-api: `scripts/toolkit-local-override.sh`, its contract test, and encounter-target examples;
- rpg-deployment: the isolated toolkit override lab, its contract test, and `LOCAL_DEV.md`;
- rpg-project: the local-game runbook and the historical toolkit-contributor sandbox plan.

Those references do not pin rpg-api to an older D&D 5e rulebook. They need repository-owned follow-ups. Project 19 was unavailable during task discovery, so this finding does not assert board state or invent unboarded companion work. They do not block deleting the unconsumed toolkit module or unblocking #1198.

— asset-pipeline agent, on behalf of KirkDiggler
EOF
```

Expected: GitHub returns the URL of one new issue comment containing the exact inventory and conservative board statement.

- [ ] **Step 6: Push and open the retirement PR**

Run:

```bash
git push -u origin feat/1215-retire-legacy-encounter

gh pr create \
  --repo KirkDiggler/rpg-toolkit \
  --base main \
  --head feat/1215-retire-legacy-encounter \
  --title 'chore!: retire the top-level legacy encounter module' \
  --body-file - <<'EOF'
## Summary

- delete the unconsumed top-level `rpg-toolkit/encounter` module with no shim
- point current toolkit guidance at the active D&D 5e encounter/resolution/session stack
- preserve legacy architecture and ADR context behind explicit retirement markers

## Why

rpg-api already runs solely on the session stack. The deleted module is the remaining source consumer of executable character/monster actions, so its retirement unblocks #1198's actions-as-data cut.

## Verification

- ordered-workspace Go source and `go.mod` consumer scan
- rpg-api `go mod why` and module-graph proof
- `make pre-commit`
- `make test-all`
- `make lint-all`
- `./scripts/check-decisions.sh`
- `./scripts/check-versions.sh`
- `git diff --check origin/main...HEAD`

Closes #1215
Unblocks #1198

— asset-pipeline agent, on behalf of KirkDiggler
EOF
```

Expected: GitHub returns one PR URL targeting `main`, with exactly one closing keyword.

- [ ] **Step 7: Force full GitHub validation and inspect initial checks**

Run:

```bash
pr_number="$(gh pr view --repo KirkDiggler/rpg-toolkit \
  --json number --jq .number)"
gh pr edit "$pr_number" --repo KirkDiggler/rpg-toolkit --add-label full-check
gh pr view "$pr_number" --repo KirkDiggler/rpg-toolkit \
  --json baseRefName,headRefName,labels,mergeable,state,url
gh pr checks "$pr_number" --repo KirkDiggler/rpg-toolkit --watch
```

Expected: base is `main`, head is `feat/1215-retire-legacy-encounter`, label list includes `full-check`, and all required checks finish successfully. Do not merge the PR; hand the exact reviewed head to Kirk.
