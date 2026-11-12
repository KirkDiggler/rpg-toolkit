# Run — 2026-11-09 — alive, not lagging S5 encounter: the compiled site is shared by content hash

Slice [rpg-project#914](https://github.com/KirkDiggler/rpg-project/issues/914) of story [rpg-project#879](https://github.com/KirkDiggler/rpg-project/issues/879), leg 1 of 3 (encounter). Base: `rpg-toolkit` `main` at `e4ac31f8`. Head: the [rpg-toolkit#2345](https://github.com/KirkDiggler/rpg-toolkit/pull/2345) branch `feat/encounter-shared-site`. Keep: keepgen 10k hexes, seed 1, 99 garrison, 20 patrols (`bench-keep-garrison`). Machine: 32 threads, load average 2.6 to 3.6.

## Shipped

- The field compiles once per content into a `Site`, keyed by the SHA-256 of its `SiteData` JSON: the field as `FieldData` writes it, and the doors' geometry written closed with no lock. — [rpg-toolkit#2345](https://github.com/KirkDiggler/rpg-toolkit/pull/2345)
- A `SiteProvider` resolves a site by hash. `MemorySites` is the in-process default: a bounded LRU map, 16 sites by default. — [rpg-toolkit#2345](https://github.com/KirkDiggler/rpg-toolkit/pull/2345)
- The checkpoint writes `site_hash` and no field. An unknown hash fails closed with `ErrUnknownSite`. A blob that carries a field and no hash is refused by name. — [rpg-toolkit#2345](https://github.com/KirkDiggler/rpg-toolkit/pull/2345)
- Door openings, partitions and the raw visible-from answers are site data. Raw answers are those for keys with no hidden face. The concealment layer stays on the instance. — [rpg-toolkit#2345](https://github.com/KirkDiggler/rpg-toolkit/pull/2345)

## Decided

- Behavior definitions stay on the instance. They ride on the roster, not on the field, so they are not in the site's hash input. A memo of compiled definitions keyed by their own bytes is the follow-up.
- The room-local sightings check stays on the instance. It decodes every observer's held testimony, which is game state.
- A door's state and lock are not part of the hash. Two games that start a door in different states share one site.
- The site piece is keyed by the hash alone. A host stores `Site.Data()` under the hash and hands it back through `LoadEncounterInput.Site` on a process that does not hold the site.
- Eviction is LRU, not refusal. An encounter already on an evicted site keeps its pointer.

## Found

Command: `RPG_MEASURE_SITE=1 go test -run TestMeasureGarrisonSite -count=1 -v .` (N=20, p50).

| row | default keep | posts-asleep keep |
|---|---|---|
| site heap (F8's missing half) | 2.67 MB (42%) | 2.63 MB (42%) |
| instance heap | 3.63 MB | 3.62 MB |
| checkpoint bytes at open | 478,677 | 495,978 |
| site piece bytes | 507,406 | 507,406 |
| LoadEncounter, site resident | 23.9 ms | 25.9 ms |
| LoadEncounter, site compiled from its piece | 47.7 ms | 48.9 ms |
| CompileSite | 23.0 ms | 21.6 ms |

Command: `go test -run XXX -bench BenchmarkGarrisonWalkLine -benchtime 10x -count 2 .` The keep is posts-asleep. Each op walks eight cells out and eight back, timed per walking step with raising steps excluded. The reloaded variants load, before each op, the checkpoint the previous op saved.

| variant (F23) | walk step | allocs/op |
|---|---|---|
| open | 11.0 ms | 588k |
| reloaded, shared site | 12.0 ms | 619k |
| reloaded, cold site | 21.2 ms | 2.16M |

- A step after a reload now costs about the same as a step on an open keep: 1.09 times. F23 had reloaded steps at about twice the open cost.
- The first bench cut reloaded the same start checkpoint every op. That measured relearning, not sweeping: 22 ms against 11 ms. The chained checkpoint is the live shape.

- The CI test budget caught a 25× regression on the first PR it ran on. `TestRegionSetupSuite` went from 0.28 s to 7.4 s under `-race`. The new Setup fingerprint walked a 4.2M-cell over-budget region before the compile could refuse it. A field over the cell budget is no longer fingerprinted, and `TestAFieldOverTheCellBudgetIsNotFingerprinted` pins it. — [rpg-toolkit#2345](https://github.com/KirkDiggler/rpg-toolkit/pull/2345)

## Mutation checks

- An instance that runs on the site's own field, and so writes the site, is killed two ways. `TestAFootprintDoorIsTheGamesOwn` fails without the race detector. `TestTwoGamesOnOneSiteRunAtOnce` reports 34 data races under `-race`.
- `ToData` dropping the hash is killed by three tests: the provider test, the checkpoint test and the reload test.
- Raw answers kept on the instance are killed by `TestTheRawAnswerIsTheSitesAfterAReload`.
- After review, Setup on a warm provider is looked up by a content fingerprint of its `FieldInput` and compiles nothing. The fingerprint is a lookup key, never the site hash. Concurrent askers for one key share a single compile. Three more mutants are killed:
  - Setup recompiling on a warm provider fails `TestTheProviderCompilesOnce`.
  - Dropping the per-key sharing fails both subtests of `TestConcurrentAskersCompileOnce`.
  - Removing the partition memo's lock fails `TestTwoGamesDeriveTheSitesPartitionsAtOnce` with 2 data races under `-race`.

## Open for legs 2 and 3

- `LoadEncounter` takes no context, so the provider is called with `context.Background()`.
- `resolution`'s own load path, used when no live world is lent, has no provider. It resolves only sites held by `DefaultSites`.

— cross-team agent, on behalf of KirkDiggler
