# Run — 2026-11-26 — NPC turn S1b the door charges a monster through the gate

Slice [rpg-project#1194](https://github.com/KirkDiggler/rpg-project/issues/1194) of story [rpg-project#1131](https://github.com/KirkDiggler/rpg-project/issues/1131). Base: `rpg-toolkit` `main` at [`1d783329`](https://github.com/KirkDiggler/rpg-toolkit/commit/1d783329).

One line per entry. Every line ends with a link.

## Shipped

- `payAtTheDoorFrom` charges a monster with `combat.Pay(combat.WithPools(mon, budget), cost.Profile)`; `chargeMonster` and `isOneReaction` are deleted — [issue 1194, design 2](https://github.com/KirkDiggler/rpg-project/issues/1194).
- A driven activation may carry a Cost; the `Input.Validate` refusal is removed — [issue 1194, design 3](https://github.com/KirkDiggler/rpg-project/issues/1194).
- Pin `rulebooks/dnd5e` v0.231.0 to v0.238.0; `go mod tidy` moved only the two go.sum lines of that module — [issue 1194, design 1](https://github.com/KirkDiggler/rpg-project/issues/1194).
- ARCHITECTURE.md, AGENTS.md and the comments of cost.go, activation.go, errors.go say what is true after the slice, with the 2026-11-26 change and the between-two-slices state — [issue 1194, design 4](https://github.com/KirkDiggler/rpg-project/issues/1194).

## Decided

- A Turn on a monster's cost is ignored and nothing is refreshed at the door; a monster's sheet gives its turn back at its own turn start — [issue 1194, design 2](https://github.com/KirkDiggler/rpg-project/issues/1194).
- A monster's cast stays refused with ErrNoPayer — [issue 1194, design 2](https://github.com/KirkDiggler/rpg-project/issues/1194).
- Tests 6 to 9 live in monster_pays_test.go as methods on the activation suite; the old driven-cost refusal test is deleted from driven_activation_test.go (moved, not rewritten in place) — [issue 1194, allowed edits](https://github.com/KirkDiggler/rpg-project/issues/1194).
- Test 9 publishes a TurnStartEvent on the bus the saved goblin sheet is loaded onto, the way the boundary publishes it — [issue 1194, test 9](https://github.com/KirkDiggler/rpg-project/issues/1194).

## Found

- F1 to F4 hold: go.mod v0.231.0; ledger.go line 25 declares the Ledger; chargeMonster and isOneReaction each have one caller; the Validate refusal is at resolve.go:201 — [Rests-on comment](https://github.com/KirkDiggler/rpg-project/issues/1194#issuecomment-7185922210).
- F5: after the pin the build is clean; vet and test compile fail on exactly contest_move_test.go:396, pause_scenes_test.go:94 and :130 (ReactionSpent) — [Rests-on comment](https://github.com/KirkDiggler/rpg-project/issues/1194#issuecomment-7185922210).
- Suite before, at the old pin (1d783329): ok, 0.98s — [base](https://github.com/KirkDiggler/rpg-toolkit/commit/1d783329).
- Edited existing tests: contest_move_test.go:396, pause_scenes_test.go:94 and :130 (ReactionSpent to SlotsLeft(reaction)); pause_test.go two tests rewritten to the new rule; cost_test.go the by-name test renamed, now ErrCannotPay; effect_test.go ErrCannotPay, comment says step 2 changes it; driven_activation_test.go the cost-refusal test removed — [issue 1194, allowed edits](https://github.com/KirkDiggler/rpg-project/issues/1194).
- cost_budget_test.go: the fixture's price is a bonus action plus "charges", which the budget owns, so a monster payer with that budget is PAID, not refused; the old case became "paid" and a "no budget" case asserts ErrCannotPay — [issue 1194, allowed edits](https://github.com/KirkDiggler/rpg-project/issues/1194).
- Proof (a) monster charged with SpendReaction again: fails the new door tests (each slot, all-or-none, nil profile, turn, budget, the gate table) and Tests 6, 7, 9 and the effect and cost refusals — [issue 1194](https://github.com/KirkDiggler/rpg-project/issues/1194).
- Proof (b) Validate refusal back: fails Tests 6, 7 and 9 (a driven cost is refused with ErrBadActivation) — [issue 1194](https://github.com/KirkDiggler/rpg-project/issues/1194).
- Proof (c) monster economy cleared when cost.Turn is set: fails TestATurnOnAMonstersCostRefreshesNothing — [issue 1194](https://github.com/KirkDiggler/rpg-project/issues/1194).
- GAP: TestCloneCharacterDataOwnsEveryMutableField/Items fails at the new pin: "fixture must populate every mutable character.Data field; add Items and its clone policy". character.Data.Items arrived between v0.231.0 and v0.238.0; the fix is in long_rest.go and long_rest_clone_test.go, neither on the slice's file list. Suite at this head: everything passes except that one — [issue 1194](https://github.com/KirkDiggler/rpg-project/issues/1194).
- The gap above was fixed on main by [rpg-toolkit#2586](https://github.com/KirkDiggler/rpg-toolkit/pull/2586) (resolution v0.82.1), and amendment 1 took the slice up again on that base, pinning v0.238.0 over main's v0.237.0 — [amendment 1](https://github.com/KirkDiggler/rpg-project/issues/1194#issuecomment-7186108767).

## Mistaken

- none observed beyond the gap above, which the issue's Rests-on facts could not have shown because F5 only builds and vets — [Rests-on comment](https://github.com/KirkDiggler/rpg-project/issues/1194#issuecomment-7185922210).
