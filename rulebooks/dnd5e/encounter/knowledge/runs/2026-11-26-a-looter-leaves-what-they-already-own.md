# Run — 2026-11-26 — item story a looter leaves what they are told to leave

Slice [rpg-project#1176](https://github.com/KirkDiggler/rpg-project/issues/1176) of story [rpg-project#850](https://github.com/KirkDiggler/rpg-project/issues/850). Base: `rpg-toolkit` `main` at [`5293b325`](https://github.com/KirkDiggler/rpg-toolkit/commit/5293b325bf83412bfd04a4c601bedaab328c3646).

## Shipped

- `LootInput.Leave`: the ids of equipped items the looter does not take; they stay on the body, untold, and a later loot takes them — [rpg-project#1176](https://github.com/KirkDiggler/rpg-project/issues/1176).

## Decided

- What is left keeps its place among what remains (original order, not id order); only what is taken is sorted by id — [rpg-project#1176 design 2](https://github.com/KirkDiggler/rpg-project/issues/1176).
- An unmatched id, an empty id and a repeated id are ignored, never an error, because the caller does not know the body — [rpg-project#1176 design 3](https://github.com/KirkDiggler/rpg-project/issues/1176).

## Found

- F1 to F4 hold at base. F3 printed `hold.go:76` (a comment), `loot.go:44`, `loot.go:45`, `loot.go:112`; nothing stores `LootInput`. Newest tag `rulebooks/dnd5e/encounter/v0.259.0` — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1176#issuecomment-7177805616).
- `LootOutput` has one field, `Items`, so the "rest of LootOutput" in Test 5 is empty; the test compares every member's beats except `taken` — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1176#issuecomment-7177805616).
- Proof 1: ignoring `Leave` fails `TestWhatTheLooterLeavesStaysOnTheBodyForTheNextPlayer`, `TestLeavingOneOfTwoTakesTheOther` and the three `TestLeaveIdsThatDoNotMatchAreIgnored` cases — [rpg-project#1176](https://github.com/KirkDiggler/rpg-project/issues/1176).
- Proof 2: clearing the body's equipment whatever was left fails the same tests (the body is still holding the cloak for the second player) — [rpg-project#1176](https://github.com/KirkDiggler/rpg-project/issues/1176).
- Suite `go test ./... -count=1 -timeout 20m` passes at base (about 76s) and at the final head (about 59s); build, vet and `golangci-lint run ./...` (0 issues) pass; `go test -race . -run 'Loot|Equipment'` 47s — [rpg-project#1176](https://github.com/KirkDiggler/rpg-project/issues/1176).

## Mistaken

- none observed — [rpg-project#1176](https://github.com/KirkDiggler/rpg-project/issues/1176).
