# Run — 2026-11-09 — the site is shared by hash through Config.Sites

processed: 2026-11-12T165555Z

Leg 2b of S5, [#914](https://github.com/KirkDiggler/rpg-project/issues/914), story [#879](https://github.com/KirkDiggler/rpg-project/issues/879). Leg 1 is the encounter's `Site` (rpg-toolkit#2345); leg 2a is resolution's provider input.

## Shipped

- `Config.Sites`, optional, nil is `encounter.DefaultSites`. Every load (`loadGivenWorld`, which every write verb, read verb and Launch go through, and `AtlasOf`) and every resolution input passes it — [sites.go](../../sites.go).
- An optional `SiteRepository` on the encounter repository (`GetSite`/`PutSite` by hash), found by type assertion like the fence. A provider miss reads the piece and hands it to `LoadEncounterInput.Site`; a miss on both fails closed with `ErrUnknownSite` beside `ErrInvalidWorld`.
- Launch stores the piece before the checkpoint that names it.

## Decided

- `PutSite` runs once per Launch, not once per hash per process: the Manager stays stateless, and the piece is content-addressed so a repeat is a no-op write. A host that wants fewer writes checks the key in `PutSite`.
- `encounter.SiteProvider` is admitted as a contract type, `SiteData` and `SiteHash` as persistence shapes (boundary_test.go): rpg-api injects the same provider the composition reads, so a session twin would be an adapter around a value nobody reshapes.
- Every load goes through one helper, `loadOnSite`, which asks the provider by hash, hands the stored piece on a miss, and maps a miss on every tier to `ErrUnknownSite` beside `ErrInvalidWorld`. The dungeon-map preview (`AtlasOf`) loads through it too.
- A site evicted between the helper's ask and the load's own ask falls through to the stored piece exactly as a first miss does, so the answer never depends on when an eviction landed (review of rpg-toolkit#2349).

## Found

- Only five session test files read `EncounterData.Field`, not the ~25 counted before the work: the rest build worlds through Launch and never touch the field.
- The checkpoint loses 37% of its bytes on the garrison keep (1.33 MB to 0.84 MB), and the piece (0.51 MB) is written once per Launch rather than on every save.
- A cold process pays about 22 ms over a warm load to compile the site from its piece (62.7 against 40.9 ms) and is warm from its second load on.

## Mistaken

- The first head, 64e7e27e, asked the provider twice per load and let an eviction between the two asks reach the host as `ErrInvalidWorld` with the reason as text, not as `ErrUnknownSite`. Fixed by the fall-through above; `TestASiteEvictedBetweenTheTwoAsksFallsThroughToTheStoredPiece` pins it.
- The same head pinned neither the Launch write order nor the preview load: moving `PutSite` after the checkpoint save, or dropping the provider from `AtlasOf`, passed every test. `TestLaunchStoresThePieceBeforeTheCheckpointThatNamesIt` and `TestTheMapPreviewLoadsThroughTheInjectedProvider` now fail on each.

## Evidence

Measured with the encounter at rpg-toolkit#2345 head `2cebdffd` and resolution at leg 2a's commit through local replaces, before the tags; the PR pins the tags those merged as (encounter v0.188.0, resolution v0.75.0) and its race suite passes on them. "Before" is origin/main `e4ac31f8`.

| row (bench-keep-garrison, 10k) | before | after |
|---|---|---|
| load, median of 9 (warm process) | 71.6 ms | 40.9 ms |
| load from the stored piece, cold process | n/a | 62.7 ms |
| Move one cell, median of 45 | 188.6 ms | 128.7 ms |
| checkpoint bytes | 1,334,360 | 836,186 |
| site piece bytes, stored once | n/a | 507,406 |

```
S5_SITE_BENCH=1 go test -count=1 -run TestSiteLoadMedian_KeepGarrison10k -v ./rulebooks/dnd5e/session/
T12_MOVE_BENCH=1 go test -count=1 -run TestMoveMedian_KeepGarrison10k -v ./rulebooks/dnd5e/session/
go test -race -count=1 ./rulebooks/dnd5e/session/...
golangci-lint run ./rulebooks/dnd5e/session/...
```

- Race suite passes; lint 0 issues.
- Mutants killed by `TestSiteSuite`: the stored piece not handed to the load; the provider not handed to the load; `PutSite` moved after the checkpoint save; the preview's site compiled outside the provider; the eviction fall-through removed.
