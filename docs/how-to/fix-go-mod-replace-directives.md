---
name: how to fix go.mod replace directives
description: Step-by-step guide for removing local replace directives; status of the remaining cases
updated: 2026-05-04
---

# How to fix go.mod replace directives

**Status (2026-05-04):**
- ✅ `items/go.mod` — directive removed (per issue #613)
- ✅ `mechanics/proficiency/go.mod` — directive removed (per issue #613)
- ⏳ `mechanics/conditions/go.mod` — directives retained; source uses old-API events symbols that no published events version exposes (tracked in #617)
- ⏳ `mechanics/spells/go.mod` — directives retained; same reason (tracked in #617)

The two cleanups that landed had no source drift — the replace directives were leftover cruft. The two that remain have real source drift: their main-branch source uses old-shape events symbols (`events.Event`, `events.HandlerFunc`, `event.Context().GetString` / `.AddModifier()`) that the current events module does not expose (events has been rewritten to a typed-topic API: `TypedTopic[T]`, `ChainedTopic[T]`, `BusEffect`). The replace directives mask that mismatch by pointing at local source. Resolving them requires **rewriting** conditions and spells against the new typed-topic API — a real refactor, not a version bump. Deferred because the 4-class playtest doesn't exercise either module.

The workspace rule (CLAUDE.md) is explicit: no replace directives on main, full stop. The two remaining cases are an active rule violation — not an "exception" — tracked in issue #617 and visible in the doc snapshot above so they don't get forgotten. Issue #613's items+proficiency portion landed; its conditions/spells portion and the CI grep guard are explicitly deferred to #617.

## The fix pattern

For each affected module:

### 1. Find the current published version of each dependency

```bash
# Check what version is published
GOPROXY=direct go list -m github.com/KirkDiggler/rpg-toolkit/core@latest
GOPROXY=direct go list -m github.com/KirkDiggler/rpg-toolkit/events@latest
# etc.
```

Or check the go.mod files of modules that already pin published versions:
```bash
cat /home/kirk/personal/rpg-toolkit/tools/spatial/go.mod
# tools/spatial has clean published pins — use these as reference versions
```

### 2. Remove replace directives and update require versions

In the affected `go.mod`, remove all `replace` blocks. Update `require` versions to match what the dependent published modules expect (NOT necessarily latest — see the warning below).

**Warning: don't blindly bump to `@latest`.** Module Version Selection picks the highest version across the dependency graph. If module A depends on B@v0.2.x (built against C@v0.1.0) and you bump A to require C@v0.6.0, B's source won't compile against C@v0.6.0. The events package split that #617 documents is exactly this case.

Reference versions to consult:
- `tools/spatial/go.mod` — clean published pins, target for the v0.6.x events world
- `mechanics/effects/go.mod` — pins events v0.1.0; matches published v0.2.1; modules in the v0.1.x world should use compatible pins

### 3. Run go mod tidy

```bash
cd /home/kirk/personal/rpg-toolkit/<module>
go mod tidy
```

This will update `go.sum` and may adjust indirect dependency versions.

### 4. Run tests

```bash
go test -race ./...
```

Tests should pass against the published versions. If they fail because the local source uses APIs that the pinned versions don't have (the events split case), you have a deeper problem than directive cleanup: the source has drifted from what its dependencies offer. That's a migration task, not a hygiene task — file a separate issue (see #617 for the worked example).

### 5. PR scope

Per the workspace rule: one issue per PR. If multiple modules can be cleaned up the same way (no source drift, just stale pins), bundling them is fine — issue #613's items + proficiency portion landed in one PR; its conditions/spells portion plus the CI grep guard rolled into #617 because they need a real source-side rewrite. If migration is needed, that's a different issue.

### 6. Verify CI passes

After pushing, ensure CI passes before merging. The key CI check is `go mod tidy` producing no diff.

## Why this matters

Local replace directives mean the module resolves its dependencies from local filesystem paths. When CI checks out only the affected module, the local paths don't exist, and the build fails. Even when CI checks out the full repo, `go mod tidy` produces a diff (the replace directives themselves) which some CI configurations treat as a failure.

The workspace CLAUDE.md is explicit: "NEVER add replace directives — Breaks CI/CD."
