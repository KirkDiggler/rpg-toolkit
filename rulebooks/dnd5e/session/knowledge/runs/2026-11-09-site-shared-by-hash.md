# Run — 2026-11-09 — the site is shared by hash through Config.Sites

Leg 2b of S5, [#914](https://github.com/KirkDiggler/rpg-project/issues/914), story [#879](https://github.com/KirkDiggler/rpg-project/issues/879). Leg 1 is the encounter's `Site` (rpg-toolkit#2345); leg 2a is resolution's provider input.

## Shipped

- `Config.Sites`, optional, nil is `encounter.DefaultSites`. Every load (`loadGivenWorld`, which every write verb, read verb and Launch go through, and `AtlasOf`) and every resolution input passes it — [sites.go](../../sites.go).
- An optional `SiteRepository` on the encounter repository (`GetSite`/`PutSite` by hash), found by type assertion like the fence. A provider miss reads the piece and hands it to `LoadEncounterInput.Site`; a miss on both fails closed with `ErrUnknownSite` beside `ErrInvalidWorld`.
- Launch stores the piece before the checkpoint that names it.

## Decided

- `PutSite` runs once per Launch, not once per hash per process: the Manager stays stateless, and the piece is content-addressed so a repeat is a no-op write. A host that wants fewer writes checks the key in `PutSite`.
- `encounter.SiteProvider` is admitted as a contract type, `SiteData` and `SiteHash` as persistence shapes (boundary_test.go): rpg-api injects the same provider the composition reads, so a session twin would be an adapter around a value nobody reshapes.
- A warm load asks the provider by hash and hands no piece; a provider that evicts between that ask and the load fails that one verb closed. Not guarded: the default holds 16 sites.

## Evidence

Measured with the encounter at rpg-toolkit#2345 head `2cebdffd` and resolution at this leg's 2a commit through local replaces; "before" is origin/main `e4ac31f8`.

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
- Mutants killed by `TestSiteSuite`: the stored piece not handed to the load; the provider not handed to the load.
