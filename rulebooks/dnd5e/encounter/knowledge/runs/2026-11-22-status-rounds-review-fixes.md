# Run — 2026-11-22 — status rounds S2b review fixes

Slice [rpg-project#1089](https://github.com/KirkDiggler/rpg-project/issues/1089) of story [rpg-project#1080](https://github.com/KirkDiggler/rpg-project/issues/1080). Base: `rpg-toolkit` `main` at [`3e7cca63`](https://github.com/KirkDiggler/rpg-toolkit/commit/3e7cca63).

One line per entry. Every line ends with a link.

## Shipped

- The read-back says "1 round" / "has passed" for one round, and "for 1 round" folded; other numbers unchanged — [rpg-project#1089](https://github.com/KirkDiggler/rpg-project/issues/1089).
- A Go-built `StatusUntilSpec` with a good number is accepted; the unread `roundsWritten` field is removed (only the grammar read it) — [rpg-project#1089](https://github.com/KirkDiggler/rpg-project/issues/1089).
- Three tests pin the timer's save: through the pieces, out of the saved record after firing, and `until` on a table row refused once — [rpg-project#1089](https://github.com/KirkDiggler/rpg-project/issues/1089).

## Decided

- Rounds judged whenever the mapping is a number and carries no other keys; every table-of-1082 refusal is unchanged because `TestDungeonspec_AStatusIsRefusedWhereItIsWrong` passes unedited — [rpg-project#1089](https://github.com/KirkDiggler/rpg-project/issues/1089).
- The Go-built test is an internal test file `dungeonspec/status_until_internal_test.go` because the grammar is unexported, as the issue allows — [rpg-project#1089](https://github.com/KirkDiggler/rpg-project/issues/1089).

## Found

- Rests on F1 to F6 all hold at `3e7cca63` (F1 lines 66-71, F2 grammar.go:927 and spec.go:1480/1514 only, F3 pieces.go:226 and 321, F4 status_lapse.go:137 and 152, F5 and F6 present) — [comment](https://github.com/KirkDiggler/rpg-project/issues/1089#issuecomment-7067214178).
- Proof, step 1 and 2: the pre-change `statuses.go` fails `TestStatusUntilOneRoundIsSaidInTheSingular`; the pre-change `grammar.go` and `spec.go` fail `TestAGoBuiltUntilIsJudgedByItsNumber` — [rpg-project#1089](https://github.com/KirkDiggler/rpg-project/issues/1089).
- Proof, step 3: removing `Lapses: d.Lapses` from `SplitPieces`, and separately `Lapses: head.Lapses` from `JoinPieces`, each fails `TestALapseSurvivesThePieces`; both reverted — [rpg-project#1089](https://github.com/KirkDiggler/rpg-project/issues/1089).
- Proof, step 4: replacing `delete(e.lapseAt, application)` in `lapseDue` fails `TestALapseFiresWhenTheWorldReachesItsRound`; reverted — [rpg-project#1089](https://github.com/KirkDiggler/rpg-project/issues/1089).
- Step 5 passes at once, as the issue says: the behaviour was right, only its proof was missing — [rpg-project#1089](https://github.com/KirkDiggler/rpg-project/issues/1089).
- Whole suite `go test ./... -count=1` green before any change and after; `golangci-lint run ./...` 0 issues; content hash pin did not move — [rpg-project#1089](https://github.com/KirkDiggler/rpg-project/issues/1089).

## Mistaken

- A `git stash` slipped into one command while setting up the before-fix proof; the permission system blocked it and nothing ran; the proof was made by swapping files from `git show` instead — [rpg-project#1089](https://github.com/KirkDiggler/rpg-project/issues/1089).
