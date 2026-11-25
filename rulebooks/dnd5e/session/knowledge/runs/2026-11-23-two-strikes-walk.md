# Run — 2026-11-23 — a note has a value S4 two strikes walk site

processed: 2026-11-25T190427Z

Slice [rpg-project#1116](https://github.com/KirkDiggler/rpg-project/issues/1116) of story [rpg-project#1007](https://github.com/KirkDiggler/rpg-project/issues/1007). Base: `rpg-toolkit` `main` at [`e5e422c0`](https://github.com/KirkDiggler/rpg-toolkit/commit/e5e422c0).

## Shipped

- `testdata/two-strikes-walk.yaml`, cut from `captain-remembers-walk.yaml` — [rpg-project#1116](https://github.com/KirkDiggler/rpg-project/issues/1116).
- `two_strikes_walk_test.go`: `TestTwoStrikesAtTheCaptain` and `TestTheSecondPersonIsBarredAtTheGateToo`, both pass on the first run — [rpg-project#1116](https://github.com/KirkDiggler/rpg-project/issues/1116).
- `go.mod`: `rulebooks/dnd5e/encounter` v0.243.0 to v0.248.0, nothing else moved — [rpg-project#1116](https://github.com/KirkDiggler/rpg-project/issues/1116).

## Decided

- Cells are copied into the new file as `strikesOutsideDoor`, `strikesBesideDoor`, `strikesDoorway` because the old test file's names live in the same package and the issue forbids editing it — [rpg-project#1116](https://github.com/KirkDiggler/rpg-project/issues/1116).

## Found

- F1 to F5 all hold at `e5e422c0`: the site and test exist, `MovementStopped` and "bars the way" are asserted at lines 187-188, the pin was v0.243.0, `NotedAs` appears 5 times in the v0.248.0 predicate.go, `MachineNotedAs` is at machine.go:746 — [Rests on comment](https://github.com/KirkDiggler/rpg-project/issues/1116#issuecomment-7089948641).
- The scene plays as the issue says: the machine hears the first lie before the row gives `warned`, so he stays; `lie-again` gives nothing and `barred` is not overwritten — [rpg-project#1116](https://github.com/KirkDiggler/rpg-project/issues/1116).
- Proof 1, post pass `not: { noted: standing }`: step 7 fails ("the warned one walks in: Guard Captain bars the way") — [rpg-project#1116](https://github.com/KirkDiggler/rpg-project/issues/1116).
- Proof 2, the `noted.as:standing` condition removed from the `in-hall` step: step 2 fails (he leaves his cell on the first lie, alice is not offered `lie-again`) — [rpg-project#1116](https://github.com/KirkDiggler/rpg-project/issues/1116).
- Proof 3, the `give` removed from `lie`'s failure row: the dungeonspec compiler refuses the site first ("nothing gives it that value"), so the suite fails at load, before step 2 — [gap comment](https://github.com/KirkDiggler/rpg-project/issues/1116#issuecomment-7089993603).
- The site dialect refuses a note value that is asked for and never given, at compile time — [gap comment](https://github.com/KirkDiggler/rpg-project/issues/1116#issuecomment-7089993603).
- Suite time with `-race`: 1.87 s (target under 5 s) — [rpg-project#1116](https://github.com/KirkDiggler/rpg-project/issues/1116).
- Whole module `go test ./... -count=1` passes at the final head (session 60.0 s); `golangci-lint run ./...` reports 0 issues — [rpg-project#1116](https://github.com/KirkDiggler/rpg-project/issues/1116).

## Mistaken

- The first proof run backed the site up to a path whose directory did not exist yet, so the file was not restored between proofs 1 and 2; the mutations stacked for one run. The file was restored from the backup and byte-compared, and proofs 1 and 3 were re-run clean — [rpg-project#1116](https://github.com/KirkDiggler/rpg-project/issues/1116).
