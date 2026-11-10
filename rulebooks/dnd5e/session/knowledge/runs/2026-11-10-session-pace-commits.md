# Run — 2026-11-10 — the continuation commits at each pace boundary

Base: `rpg-toolkit` `main` at 56ab0946 (session v0.178.0, encounter v0.192.0). Story: [rpg-project#879](https://github.com/KirkDiggler/rpg-project/issues/879), card [rpg-project#926](https://github.com/KirkDiggler/rpg-project/issues/926) (S13, ruling R7 option D), PR 1 of 2. No encounter, protos, Lua, key or web change.

## Shipped

- The walk's third boundary, `paceStopAfter`, beside "walk through" and "stop before due" (the answer's). Before each step it asks `PaceDue`, the shared rule; after a due step that moved it returns at the next index, since the raise already ran inside that `Step`. The walk's last step never stops.
- `continueWalk` loops: walk to a boundary, save the walker's progress, `commitPace`, walk on; the last commit is the final `commit`.
- `commitAt(final)`. Settlement, the record batch and the act batch all read from one commit cursor, `writeScope.deltaFrom`. That is the act's baseline until a commit lands, then `writeScope.committed`, the first seq the landed save did not hold. The cursor moves only on a landed save.
- `CloseAct` and the fence's saved flag on the final commit only. `persistFenced` no longer sets the flag; LevelUp's one-save path sets it itself.
- `ContinuationReport.Commits` and `SavedThrough`. On a failure after a landed commit, `Continue` returns that report beside the error; with none landed it returns nil as before.
- Docs: the act doc's "a run never spans a commit" gains its second reason; the fence's saved flag; the settlement and experience docs read the cursor; the README names the walk's commits.

## Decided

- The save path is the existing pieces path, unchanged: `ToCheckpoint(Since)`, `SplitPieces(Held)`, one fenced save, which already advances version, record baseline and marks per landed save.
- Stale mid-continuation fails closed and never retries; landed commits stand and were delivered.
- The pace commit settles its own falls: a fall saved without its grant would be past every later cursor.
- `commitRecord` returns the cursor's next value from the read it already makes, so a commit costs no extra record read.
- The boundary is the `PaceDue` prediction asked before the step, not a fact from `StepOutput`. A wrong prediction costs one commit too many or too few, never a lost or doubled beat.

## Found

- The acts reach the stream per commit and per recipient, so a two-commit walk interleaves recipients differently from a whole walk. Per listener the sequence is identical. `TestAContinuedWalkDeliversWhatAWholeWalkDelivers` now compares per recipient.
- The rest now asks `PaceDue` before each of its steps, so the delivery test's sheet-read count grows by the rest's step count.
- The walk segment already split at the tick in a whole walk. The tick is a clock entry audienced to the whole roster, so every walk run is closed before the pace commit.

## Mistaken

- None caught after the first draft; all three mutants below failed the suite on the first run.

## Verification

- `go test -race -count=1 ./...` in `rulebooks/dnd5e/session`: green. `golangci-lint run ./...`: 0 issues.
- New: `TestPaceCommitSuite` (the continuation hall over plain, fenced-only, appending-only and pieces hosts; about 0.5 s under race) and `TestPaceCommitInternalSuite` (a fenced tomb, one scope; about 0.1 s).
- Mutant: commit the record and acts from `scope.baseline` instead of the cursor. It fails the delivery headline on all four hosts, the two-raise walk, catch-up across a pace commit and the continuation delivery test.
- Mutant: set the saved flag on a pace commit. It fails `TestASheetWrittenAfterAPaceCommitRidesTheNextSave`.
- Mutant: settle from `scope.baseline`. It fails `TestAFallIsSettledInThePaceCommitThatSavesIt` (the fall paid twice).
