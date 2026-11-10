# Run — 2026-11-10 — S6 PR 2: the session keeps a write verb's encounter resident, checked by one version read

Story [rpg-project#879](https://github.com/KirkDiggler/rpg-project/issues/879) "Alive, not lagging", slice S6 card [rpg-project#927](https://github.com/KirkDiggler/rpg-project/issues/927), PR 2 of 3. Base: `rpg-toolkit` `main` at [`178d11f4`](https://github.com/KirkDiggler/rpg-toolkit/commit/178d11f4). Encounter pinned at the pushed PR 1 commit [`3c3a5ddb`](https://github.com/KirkDiggler/rpg-toolkit/commit/3c3a5ddb) (rpg-toolkit#2367).

## Shipped

- `Config.Residency`, the `Residency` interface (`Take`, `Keep`), the opaque `*Resident` handle, and `NewResidency` / `MemoryResidency`: in process, bounded by entries and by estimated bytes, least recently kept evicted first. — [rpg-project#927](https://github.com/KirkDiggler/rpg-project/issues/927)
- `CheckpointVersionReader`, an optional capability found by type assertion on `Config.Encounters` beside the fence. A host without it never touches the residency. — [rpg-project#927](https://github.com/KirkDiggler/rpg-project/issues/927)
- The verb path. `openScope` takes the copy on every fenced open, Launch included. For a stored world it reads the version once; on a match it rebinds the copy and runs on it with the fence it carries, and otherwise it drops the copy and loads. `commitAt`'s final commit stages the copy after its save landed and its batches went out. The guard's release keeps what was staged, beside the pending-writes drop. — [rpg-project#927](https://github.com/KirkDiggler/rpg-project/issues/927)
- `residency_test.go` and `residency_internal_test.go`: the warm skip, another writer's save, each drop rule, a host without the reader, the bound, and the session-level equivalence. — [rpg-project#927](https://github.com/KirkDiggler/rpg-project/issues/927)

## Decided

- Keyed by session id, because the guard the copy rides is the session's. The handle carries the encounter id, and a record naming another encounter never uses it. — [rpg-project#927](https://github.com/KirkDiggler/rpg-project/issues/927)
- The stage runs after delivery, not at the save. A host stream that panics after the save then keeps nothing. Every host collaborator a verb calls runs before that point. — [rpg-project#927](https://github.com/KirkDiggler/rpg-project/issues/927)
- The release also refuses a staged copy whose record moved past the fence's `recordNext`. That copy is not what landed. — [rpg-project#927](https://github.com/KirkDiggler/rpg-project/issues/927)
- The handle carries the standing's member kinds. A load reads them off the stored world, placed and waiting, and a kept copy skips that read. — [rpg-project#927](https://github.com/KirkDiggler/rpg-project/issues/927)
- A copy that cannot be rebound is dropped and the world loads; it does not fail the verb. — [rpg-project#927](https://github.com/KirkDiggler/rpg-project/issues/927)
- `GetCheckpointVersion(ctx, encounter) (uint64, error)` as the card names it. An error or a different version is a miss. — [rpg-project#927](https://github.com/KirkDiggler/rpg-project/issues/927)
- Bytes are estimated from the record's next seq: 1.6 MB fixed and 0.9 KB per entry, the map's measure. Defaults are 64 entries and 512 MiB; a negative bound keeps none. — [rpg-project#927](https://github.com/KirkDiggler/rpg-project/issues/927)
- The tests use the small keep without its garrison, its creatures made neutral to the party. The garrison's Launch costs 6 s under `-race`, and the plain keep's costs 1 s. — [rpg-project#927](https://github.com/KirkDiggler/rpg-project/issues/927)

## Found

- The in-memory pieces store joins a save before it checks the fence. A stale verb's segment overlapped the other writer's and failed as invalid log data, not `ErrStaleCheckpoint`. The test store checks the fence first, as a host's one script does. — [rpg-project#927](https://github.com/KirkDiggler/rpg-project/issues/927)
- The differential compared only the stored world at first, so a warm verb with a wrong act baseline passed: the store was right and the acts delivered were not. It now also compares the pieces each save carries and the acts each verb delivered. — [rpg-project#927](https://github.com/KirkDiggler/rpg-project/issues/927)

Commands, from `rulebooks/dnd5e/session`:

```
go test -race -count=1 -run 'TestMemoryResidency|TestAWarm|TestAnother|TestAStale|TestARefusedVerb|TestACanceled|TestAPanic|TestAClosed|TestAContinuationThatFails|TestAHostWithout|TestBoundary' -v .
go test -count=1 ./...
```

| test, under `-race` | local |
|---|---|
| `TestAWarmVerbSkipsTheLoad` (pays the keep's compile) | 2.39 s |
| `TestAWarmRunWritesWhatAColdRunWrites` | 4.22 s |
| every other residency test | under 1 s |

Mutants, each applied alone and restored:

| mutant | caught by |
|---|---|
| keep before the save result (stage before `persist`) | every residency test |
| skip the version read | `TestAnotherWritersSaveDropsTheCopy`, `TestAStaleSaveKeepsNoCopy`, `TestACanceledVerbKeepsNoCopy`, `TestAWarmVerbSkipsTheLoad` |
| keep after a stale save | survives alone: the release's record check drops the copy, whose record moved past what landed. With that check also removed, `TestAStaleSaveKeepsNoCopy` fails. |
| the record check removed alone | survives: the stage after a landed save is the primary guard |
| the standing's kinds dropped | `TestAWarmVerbSkipsTheLoad`, `TestAnotherWritersSaveDropsTheCopy`, `TestAWarmRunWritesWhatAColdRunWrites` |
| the act baseline set to 1 | `TestAWarmRunWritesWhatAColdRunWrites` (the acts delivered) |
| the piece marks dropped | `TestAWarmRunWritesWhatAColdRunWrites` (the pieces saved) |

## Mistaken

- The plain keep's monsters are hostile, so the party launched into a fight and a Move needed a declaration. The fixture makes them neutral, as the garrison is. — [rpg-project#927](https://github.com/KirkDiggler/rpg-project/issues/927)

## Verification

- The residency tests and the boundary suite pass under `-race`; the module's `go test ./...` passes; `golangci-lint run ./...` reports 0 issues.
