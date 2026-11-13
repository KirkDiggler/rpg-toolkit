# Run — 2026-11-09 — the session saves the record appended

processed: 2026-11-12T165555Z

Base: `rpg-toolkit` `main` at 09147992. Story: [rpg-project#879](https://github.com/KirkDiggler/rpg-project/issues/879), card [rpg-project#897](https://github.com/KirkDiggler/rpg-project/issues/897) leg (c), PR 3 of 4. Upstream: record [#2352](https://github.com/KirkDiggler/rpg-toolkit/pull/2352) and encounter [#2353](https://github.com/KirkDiggler/rpg-toolkit/pull/2353), developed against their pushed commits and pinned at their tags, play/record v0.4.0 and encounter v0.189.0.

## Shipped

- `AppendingEncounterRepository`, an optional capability found by type assertion on `Config.Encounters` beside the fence. `SaveAppended` takes `SaveAppendedInput`: the embedded `SaveFencedInput` with `Data` as the rest, plus `Appended`, `Since` and `Floor`.
- `checkpointFence.recordNext`: the first record seq the store lacks. It is set from the loaded log on a fenced read, 1 at Launch, and advanced to the rest's NextSeq only after a landed save.
- `persist` takes the world at the storage boundary itself: `ToCheckpoint(Since: recordNext)` on an appending host, `ToData` otherwise. The appending path never copies the record's older entries.
- LevelUp's seated save, which writes the world back as read, splits the stored world the same way and appends nothing.
- `record.LogData` admitted to the boundary's persistence shapes, with its reason.

## Decided

- Nothing was added to `SaveFencedInput`, because a host that ignored a new field would write a rest-only world and lose the record silently.
- The baseline lives on the fence, not the act's `baseline`, because the act's baseline must stay fixed while later per-pace commits move the stored-through mark several times in one verb.
- The fence's version also advances after a landed save, beside `recordNext`, so a later second save in one verb names the right version.
- Reads stay whole, because assembly is the store's job and every load path already takes one `EncounterData`.

## Found

- The boundary test caught `record.LogData` on the new input. It is the same bytes `EncounterData.Log` already carried, so it joined the persistence shapes rather than getting a twin.
- The record moved `DataFrom` and `JoinData` to Input structs during review. The session had one production call site (`splitStored`) and one test helper to update at the re-pin, and the test store now passes its floor to the join.

## Mistaken

- Review of #2354 at c56ff0c4: the floor was never pinned, so a save that forced `Floor: 0` survived. The test store now drops segments below the floor its save names, and a bounded run's joined record must start at that floor; the mutant fails it.
- The same review: `encounter.ErrBadSince` fell through translate's default arm. It now maps to `ErrBadRepository`, because the session reads Since from the stored record.
- The run record first landed in the repo-root `knowledge/runs`; it belongs to the module.

## Verification

- `go test -race -count=1 ./...` in `rulebooks/dnd5e/session`: green, 250 s for the package, with no new test over the budget.
- `golangci-lint --new-from-rev origin/main`: 0 issues.
- New suite `TestAppendedRecordSuite` and `TestRecordNextMovesOnlyWithALandedSave` pin the behaviour listed in the PR body.
