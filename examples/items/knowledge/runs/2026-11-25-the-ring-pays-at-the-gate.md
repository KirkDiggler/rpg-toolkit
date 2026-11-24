# Run — 2026-11-25 — one way to pay for an ability S1160 the ring pays at the gate

Slice [rpg-project#1160](https://github.com/KirkDiggler/rpg-project/issues/1160) of story [rpg-project#1131](https://github.com/KirkDiggler/rpg-project/issues/1131). Base: `rpg-toolkit` `main` at [`64c707aa`](https://github.com/KirkDiggler/rpg-toolkit/commit/64c707aa).

## Shipped

- `Item.Budget(holderID)` and `Budget.HasPool`, `PoolLeft`, `SpendPool` in `budget.go`, no rulebook import — [issue](https://github.com/KirkDiggler/rpg-project/issues/1160).
- `payRing` in the fixture pays `combat.Pay(combat.WithPools(economy, item.Budget(holder)), definition.Cost)` in one call, then resolves the healing; `useRing` is unchanged — [issue](https://github.com/KirkDiggler/rpg-project/issues/1160).
- The runnable example uses `payRing` and prints `bonus actions left: 0` after `charges after use: 0` — [issue](https://github.com/KirkDiggler/rpg-project/issues/1160).
- README: a paragraph on paying at the gate and a Candidate API entry for `Item.Budget` — [issue](https://github.com/KirkDiggler/rpg-project/issues/1160).
- Tests in `gate_pay_test.go`: one payment, spent bonus action, empty ring, other holder, detached, two rings, undeclared pool, save/transfer/rest — [issue](https://github.com/KirkDiggler/rpg-project/issues/1160).

## Decided

- `SpendPool` calls `Resource.Use` and ignores its refusal, so a spend above what is left changes nothing and never goes below zero, as the issue's words say — [issue](https://github.com/KirkDiggler/rpg-project/issues/1160).
- `seam_test.go` allows `core/resources`, the one edit the issue permits — [issue](https://github.com/KirkDiggler/rpg-project/issues/1160).

## Found

- F1 to F6 all hold at `64c707aa`: go.mod printed `v0.230.0`; `PoolSource` and `WithPools` at tag v0.231.0; `NewActionEconomy` and `BonusActionsRemaining`; three Item fields; the seam test's allowed map; the example's expected output — [comment](https://github.com/KirkDiggler/rpg-project/issues/1160#issuecomment-7146156033).
- go.mod diff: `rulebooks/dnd5e v0.230.0` to `v0.231.0`; go.sum swaps the matching h1 line; nothing failed after the pin alone (51 tests, ok) — [issue](https://github.com/KirkDiggler/rpg-project/issues/1160).
- Existing tests edited: `seam_test.go` (allowed import) and `example_test.go` (Design 4: uses `payRing`, gains one output line) — [issue](https://github.com/KirkDiggler/rpg-project/issues/1160).
- Proofs: `SpendPool` doing nothing fails Test 1 and four more; `PoolLeft` ignoring the holder fails `TestRingHeldBySomeoneElseCannotPay` — [issue](https://github.com/KirkDiggler/rpg-project/issues/1160).
- Test count 51 before, 59 at the final head; suite ok; race ok — [issue](https://github.com/KirkDiggler/rpg-project/issues/1160).

## Mistaken

- none observed — [issue](https://github.com/KirkDiggler/rpg-project/issues/1160).

— cross-team agent, on behalf of KirkDiggler
