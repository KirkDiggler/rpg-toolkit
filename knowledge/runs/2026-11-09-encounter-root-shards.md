# Run — 2026-11-09 — the encounter module runs as three CI shards, without cover

processed: 2026-11-12T165555Z

Base: `rpg-toolkit` `main` at b7ac2a42. Story: [rpg-project#907](https://github.com/KirkDiggler/rpg-project/issues/907), slice [rpg-project#908](https://github.com/KirkDiggler/rpg-project/issues/908). Closes [#2335](https://github.com/KirkDiggler/rpg-toolkit/issues/2335).

## Headline

Atomic cover, not the runner, was most of the encounter root package's CI time. No workflow step uploads or reads `coverage.txt`, so CI no longer produces it for any module.

Local wall times, 32 cores, load 6–12 from parallel agents, `-race -count=1`:

| shard | with atomic cover | without cover |
|---|---|---|
| heavy (root, `-run "$RE"`) | 230 s | 82 s |
| rest (root, `-skip "$RE"`) | 265 s | 89 s |
| subpackages | 243 s | 228 s |

- The root package goes from 495 s to 171 s with cover dropped. The race-only inventory in #2335 was 188 s on the same box, so the "CI is 3x the dev box" figure was mostly race plus atomic cover against race alone.
- Subpackages are not cover-bound: `dungeonspec/keepgen` takes 234 s with cover and 228 s without. It is now the longest shard and the next target, with #2236.

## Shipped

- The "Check module" step runs `go test -race` with no `-coverprofile` for every module. Coverage stays a local command: `make coverage` for the modules it lists, or `go test -race -coverprofile=coverage.txt -covermode=atomic ./...` in a module.
- `test-changed` builds its matrix as an `include` list in the `changes` job: one `{module, shard: all}` per changed module, three entries for `rulebooks/dnd5e/encounter`.
- `heavy` runs the root package with `-run "$ENCOUNTER_HEAVY_TESTS"`. `rest` runs it with `-skip "$ENCOUNTER_HEAVY_TESTS"` plus fmt, tidy and lint. `subpackages` runs every other encounter package unfiltered.
- The regex is one workflow `env` value, so `-run` and `-skip` cannot drift. Each job writes its test time to the job summary.
- Every test command keeps the `-timeout 25m` from [#2336](https://github.com/KirkDiggler/rpg-toolkit/pull/2336), merged into this branch.

## Decided

- Three shards stay even though the root package now fits in one job. They are insurance against the next growth.
- Subpackages are their own shard. Run after the root, they would double that job's time. Splitting them further stays with [#2236](https://github.com/KirkDiggler/rpg-toolkit/issues/2236).
- `subpackages` stays unfiltered, so a subpackage test that shares a heavy name still runs.

## Found

`RE='^(TestObserversOf_EqualsFullRefresh|TestTheGarrisonsDoorOpeningsAreTheWholeFloorScans|TestPlacedReach_AgreesWithTheWalk)$'`

```
cd rulebooks/dnd5e/encounter
go test -race -count=1 -timeout 25m -run "$RE" .
go test -race -count=1 -timeout 25m -skip "$RE" .
go test -race -count=1 -timeout 25m $(go list ./... | grep -vxF "$(go list .)")
# "with cover" rows add -coverprofile=… -covermode=atomic
```

- Partition check with `go test -list '.*' .`: 526 top-level names, 3 match the regex, 523 do not. With `TestPlacedReach_AgreesWithTheWalk` renamed, 2 match and 524 do not, and the renamed test falls into `rest`.
- A workflow-only change sets `should_run_all`, so this PR runs `test-all` (`make test-all`: race, no cover, no shards), not `test-changed`. The first encounter PR to merge this main exercises the shards.
