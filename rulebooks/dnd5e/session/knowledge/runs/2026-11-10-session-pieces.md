# Run — 2026-11-10 — the session saves the rest as pieces

processed: 2026-11-12T165555Z

Base: `rpg-toolkit` `main` at 79312d70. Story: [rpg-project#879](https://github.com/KirkDiggler/rpg-project/issues/879), card [rpg-project#897](https://github.com/KirkDiggler/rpg-project/issues/897) leg (b), PR 3 of 4. Upstream: encounter [#2362](https://github.com/KirkDiggler/rpg-toolkit/pull/2362) (`SplitPieces`/`JoinPieces` with marks), developed against its pushed commit 8d4793d8 by pseudo-version, then pinned at its tag v0.192.0 (main 3b4e523b).

## Shipped

- `PiecesEncounterRepository`, an optional refinement of `AppendingEncounterRepository`, found by type assertion on `Config.Encounters`. `SavePieces` takes `SavePiecesInput`: the embedded `SaveAppendedInput` (segment, Since, Floor, session record, sheets, seats, Expected) with `Data` nil, plus `Pieces`, `Removed` and `Marks`.
- `GetFencedEncounterOutput.Marks`: the marks the store holds beside its pieces, nil when the host keeps none.
- `checkpointFence.marks` beside `recordNext`: the fenced read's marks, nil at Launch, replaced by the save's new marks only after it lands.
- The persist path with the capability: `ToCheckpoint(Since)`, then `SplitPieces(Rest, Held: marks)`, then one `SavePieces`. LevelUp's seated write-back splits the stored world against the loaded marks and sends the head alone.
- Fallback order unchanged when the capability is absent: appending, fenced, plain.
- `encounter.Piece`, `encounter.PieceKey` and `encounter.Mark` admitted to the boundary's persistence shapes, with their reasons.

## Decided

- The save input embeds the appending input, which embeds the fenced one, as the lead decided; `Data` is set nil so a host that reads it fails loudly rather than writing a whole rest beside the pieces.
- `Marks` on the input is the whole set the session will hold, though the store already has each sent piece's mark on the piece; a host may check or cache it.
- Nothing was added to the existing inputs.

## Found

- The boundary test (S2) caught the piece types on the exported surface. They are the rest's bytes by shape, minted by the encounter, so they joined the persistence shapes.
- Two runs of the same verbs differ only in minted ids (entry wire ids, the act correlation and its tag), so the pieces-versus-whole load test masks exactly those.
- The small keep garrison costs about 2.7 s to compile and about 6 s to launch under race; its suite carries an allowlist line, and every other case runs on small authored halls.

## Mistaken

- The first draft put every case on the keep (35 s under race); the hall suite now covers launch, removal, staleness, the level and the fallback in 0.2 s.
- A skeleton in sight of the twin hall started a fight, and a Move then needs a declaration; the hall now has two players watching each other.

## Verification

- `go test -race -count=1 ./...` in `rulebooks/dnd5e/session`: green except the boundary test before the persistence shapes were admitted; the boundary suite is green after.
- `golangci-lint run ./...`: 0 issues.
- Mutants: advancing the marks before the save's result fails `TestMarksMoveOnlyWithALandedSave`; splitting with `Held: nil` fails it and the hall suite's removal and level cases.
- New: `TestPiecesCheckpointSuite` (hall, 0.2 s), `TestPiecesKeepSuite` (small keep oracle, 13 s under race, allowlisted), `TestMarksMoveOnlyWithALandedSave`.
