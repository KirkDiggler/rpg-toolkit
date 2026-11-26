# Run — 2026-11-26 — NPC turn 1c-1 the rules price a monster's action

Slice [rpg-project#1197](https://github.com/KirkDiggler/rpg-project/issues/1197) of story [rpg-project#1131](https://github.com/KirkDiggler/rpg-project/issues/1131). Base: `rpg-toolkit` `main` at [`7c42d953`](https://github.com/KirkDiggler/rpg-toolkit/commit/7c42d953).

## Shipped

- `monster.CostOfAction` (a listed action's own stated cost, copied, else one action) and `monster.CostOfDisengage` (bonus action with Nimble Escape, else one action), in `monster/spend_profile.go` with seven tests — [rpg-project#1197](https://github.com/KirkDiggler/rpg-project/issues/1197).

## Decided

- A listed action that states no cost costs one action and a multiattack is one listed action, so its swings are not priced here — [rpg-project#1197 design](https://github.com/KirkDiggler/rpg-project/issues/1197).
- A sheet without Nimble Escape pays one action for Disengage, since a price function must answer for every sheet and "free" would be wrong — [rpg-project#1197 design](https://github.com/KirkDiggler/rpg-project/issues/1197).
- An unreadable stored trait wraps the read's own error with the monster id and is not a price — [rpg-project#1197 design](https://github.com/KirkDiggler/rpg-project/issues/1197).

## Found

- F1 to F5 held at `7c42d953`; `CloneSpendProfile` is at `combat/actions/definition.go:129`, and no bestiary definition states a cost (the grep printed nothing); the multiattack monster used is the goblin boss — [rests-on comment](https://github.com/KirkDiggler/rpg-project/issues/1197#issuecomment-7186216037).
- `monster` importing `refs` and `combat/actions` makes no cycle; `go build ./...` is clean — [rests-on comment](https://github.com/KirkDiggler/rpg-project/issues/1197#issuecomment-7186216037).
- Proof a (return the definition's own pointer): the cost-of-its-own test fails with "Expected and actual point to the same object" — [rpg-project#1197](https://github.com/KirkDiggler/rpg-project/issues/1197).
- Proof b (every Disengage a bonus action): the Disengage test fails with "Not equal" — [rpg-project#1197](https://github.com/KirkDiggler/rpg-project/issues/1197).
- Proof c (nil cost when none stated): the one-action test, the gate-takes-it test and the Disengage-then-scimitar test fail with "Expected value not to be nil" — [rpg-project#1197](https://github.com/KirkDiggler/rpg-project/issues/1197).
- The suite in `rulebooks/dnd5e` passes before the change and at the final head; `go build`, `go vet` and `golangci-lint run` are clean — [rpg-project#1197](https://github.com/KirkDiggler/rpg-project/issues/1197).

## Mistaken

none observed — [rpg-project#1197](https://github.com/KirkDiggler/rpg-project/issues/1197).
