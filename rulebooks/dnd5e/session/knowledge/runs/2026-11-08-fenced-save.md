# Run — 2026-11-08 — the save is fenced on a checkpoint version

Leg (a) of S12, [#897](https://github.com/KirkDiggler/rpg-project/issues/897), story [#879](https://github.com/KirkDiggler/rpg-project/issues/879).

## Shipped

- An optional `FencedEncounterRepository`, found by type assertion on `Config.Encounters`: the write load reads the world with the store's checkpoint version, and one `SaveFenced` carries the world, the touched session record, and every sheet and seat the verb wrote, landing only if that version still holds — [session PR](https://github.com/KirkDiggler/rpg-toolkit/pull/2322).

## Decided

- The version is a `uint64` the store increments, not an opaque string: a Redis `INCR` answers an integer, comparison is exact, and 0 means no checkpoint yet. "Unknown" is a separate state, an unheld fence, which a plain repository gives — [checkpoint.go](../../checkpoint.go).
- The session record rides in the fenced save instead of the two calls the brief named: two calls cannot give the host one transaction across both blobs, and the brief's guarantee (a stale save never writes the session record) holds trivially — [PR](https://github.com/KirkDiggler/rpg-toolkit/pull/2322).
- Sheets and seats ride the fenced save too, the review's blocking finding and the design's law (one fenced transaction of the pieces that changed). They are held in a pending overlay the sheet store and seat read consult first, so the many read sites that hold no scope still read the verb's own writes; releasing the session's guard drops what was not saved. A seated LevelUp, which opens no run, saves through a fenced save of the run holding the character — [review](https://github.com/KirkDiggler/rpg-toolkit/pull/2322#pullrequestreview-5819453931).
- The plain path stays today's writes; an unseated character's sheet under its own guard stays a plain write and its guard must still not expire.

## Found

- The record's `NextSeq` cannot be the fence: SetSneaking to the mode already held commits with no beat, and the fence test pins that the version still moves.
- `saveWalkProgress` writes the walker's sheet before the world save only because persist saves the world last; it is not crash insurance. Under the fence the walk's spent movement is part of the commit.

## Mistaken

- The first head (`abe88d1c`) fenced the world and the session record only, while its locker sentence told hosts an expiring lease was safe. A stale fight Move still landed the walker's sheet. Fixed here; the probe is now `TestAStaleFightMoveWritesNoSheet`.

## Evidence

- `go test -race -count=1 ./rulebooks/dnd5e/session/...` passes.
- `golangci-lint run ./rulebooks/dnd5e/session/...`: 0 issues.
- Mutants killed by `TestCheckpointFenceSuite` and `TestPendingWritesSuite`: loaded version replaced by 1; sheets written plain under the fence; seats written plain; release keeping unsaved writes; the sheet load ignoring the overlay; persist skipping the fence.
