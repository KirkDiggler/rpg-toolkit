# Run — 2026-11-09 — the encounter module runs as three CI shards

Base: `rpg-toolkit` `main` at b7ac2a42. Story: [rpg-project#907](https://github.com/KirkDiggler/rpg-project/issues/907), slice [rpg-project#908](https://github.com/KirkDiggler/rpg-project/issues/908). Closes [#2335](https://github.com/KirkDiggler/rpg-toolkit/issues/2335).

## Shipped

- `test-changed` builds its matrix as an `include` list in the `changes` job: one `{module, shard: all}` per changed module, three entries for `rulebooks/dnd5e/encounter`.
- `heavy` runs the root package with `-run "$ENCOUNTER_HEAVY_TESTS"`, without cover.
- `rest` runs the root package with `-skip "$ENCOUNTER_HEAVY_TESTS"`, with cover, plus fmt, tidy and lint for the module.
- `subpackages` runs every other encounter package unfiltered, with cover.
- The regex is one workflow `env` value, so `-run` and `-skip` cannot drift. Each job writes its test time to the job summary.
- The shard commands carry `-timeout 25m`. The `all` path is unchanged; [#2336](https://github.com/KirkDiggler/rpg-toolkit/pull/2336) gives every module the same budget.

## Decided

- Subpackages are their own shard. Run after the root in `rest`, they made that job about 508 s locally; `dungeonspec/keepgen` alone is 234 s. This is the cheap part of [#2236](https://github.com/KirkDiggler/rpg-toolkit/issues/2236)'s by-package shape; splitting the subpackages further stays with #2236.
- `heavy` drops cover. No step uploads or reads `coverage.txt`, and atomic cover nearly triples those three tests.
- `subpackages` stays unfiltered, so a subpackage test that shares a heavy name still runs.

## Found

Local times, 32 cores, load 7–12 from parallel agents, `-race -count=1`:

| shard | command | test time | wall |
|---|---|---|---|
| heavy, atomic cover | `go test -race -run "$RE" -coverprofile=… -covermode=atomic .` | 198 s | 230 s |
| heavy, no cover (shipped) | `go test -race -timeout 25m -run "$RE" .` | 80 s | 82 s |
| rest | `go test -race -timeout 25m -skip "$RE" -coverprofile=… -covermode=atomic .` | 263 s | 265 s |
| subpackages | `go test -race -timeout 25m -coverprofile=… -covermode=atomic $(go list ./... \| grep -vxF "$(go list .)")` | max 234 s (`dungeonspec/keepgen`) | 243 s |

`RE='^(TestObserversOf_EqualsFullRefresh|TestTheGarrisonsDoorOpeningsAreTheWholeFloorScans|TestPlacedReach_AgreesWithTheWalk)$'`

- Atomic cover is the dominant cost of the root package under race: the race-only inventory in #2335 was 188 s, and heavy plus rest with cover is 461 s. The `rest` shard is now the longest, and cover is most of it.
- Partition check with `go test -list '.*' .`: 526 top-level names, 3 match the regex, 523 do not. With `TestPlacedReach_AgreesWithTheWalk` renamed, 2 match and 524 do not, and the renamed test falls into `rest`.
- A workflow-only change sets `should_run_all`, so this PR runs `test-all` (`make test-all`: race, no cover, no shards), not `test-changed`. The first encounter PR to merge this main exercises the shards.
