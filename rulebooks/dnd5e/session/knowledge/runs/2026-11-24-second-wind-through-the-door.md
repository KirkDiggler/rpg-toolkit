# Run — 2026-11-24 — one way to pay for an ability S4 second wind through the door

Slice [rpg-project#1147](https://github.com/KirkDiggler/rpg-project/issues/1147) of story [rpg-project#1131](https://github.com/KirkDiggler/rpg-project/issues/1131). Base: `rpg-toolkit` `main` at [`879406d1`](https://github.com/KirkDiggler/rpg-toolkit/commit/879406d1).

One line per entry. Every line ends with a link.

## Shipped

- `Manager.Activate` runs an ability that carries a description through `resolution.NewAction` and pays its cost at the door; every other feature keeps `NewActivation` with no cost; the "Nothing is charged here" comment says which is which — [https://github.com/KirkDiggler/rpg-project/issues/1147](https://github.com/KirkDiggler/rpg-project/issues/1147).
- Pins: `rulebooks/dnd5e` v0.226.0 to v0.228.0 and `rulebooks/dnd5e/resolution` v0.80.0 to v0.81.0; encounter stays v0.255.0; no other requirement line moves — [https://github.com/KirkDiggler/rpg-project/issues/1147](https://github.com/KirkDiggler/rpg-project/issues/1147).
- `testdata/second-wind-walk.yaml` and `SecondWindWalkSuite` play the four-step scene, with a reload after the heal — [https://github.com/KirkDiggler/rpg-project/issues/1147](https://github.com/KirkDiggler/rpg-project/issues/1147).
- The selector projection gains `Effect`; the guard classifies `Definition.Effect`, `EffectProfile.To` and `EffectProfile.Healing` as mechanical — [Amendment 1](https://github.com/KirkDiggler/rpg-project/issues/1147#issuecomment-7122190553).

## Decided

- The effect is part of an action's identity (mechanical) — [Amendment 1](https://github.com/KirkDiggler/rpg-project/issues/1147#issuecomment-7122190553).
- A used Second Wind's row refusal is the pool's shortfall (no_budget, currency charges), accepted — [Amendment 1](https://github.com/KirkDiggler/rpg-project/issues/1147#issuecomment-7122190553).
- The owner named on the "Fighter level" modifier is accepted; the door's cannot-pay refusal is pinned in the resolver, step 3(b) becomes the went-through-the-door test — [Amendment 2](https://github.com/KirkDiggler/rpg-project/issues/1147#issuecomment-7122307741).
- The cost is built the way `priceSwing` (economy.go) builds it, with the profile cloned as `cast.go` clones it — [Amendment 2](https://github.com/KirkDiggler/rpg-project/issues/1147#issuecomment-7122307741).

## Found

- After the pins alone, tests failed to compile (Second Wind's removed `Uses`/`MaxUses`) in activation_events_test.go and join_long_rest_test.go, and rest_test.go failed on the removed `uses` key — [gap 1](https://github.com/KirkDiggler/rpg-project/issues/1147#issuecomment-7122180391).
- After the pins alone, `TestSelectorProjectionClassifiesEveryDefinitionField` failed on three new Definition fields — [gap 1](https://github.com/KirkDiggler/rpg-project/issues/1147#issuecomment-7122180391).
- After the pins alone, `TestWorldClockDoorsSuite/TestAFreeRoamActivationIsAdmitted` failed: the used row says no_budget in currency charges, not "uses remaining" — [gap 2](https://github.com/KirkDiggler/rpg-project/issues/1147#issuecomment-7122180783).
- A second use is refused as `ErrStaleDeclaration` before and after the change; the door's own refusal is unreachable through `Activate` — [gap 3](https://github.com/KirkDiggler/rpg-project/issues/1147#issuecomment-7122220733).
- The recorded heal's "Fighter level" modifier gains `SourceID` = the actor; every other field of the output and the recorded beat is identical for a fixed roll (identity, amount 13, requested 13, before 10, after 23, total 13, dice trace) — [gap 4](https://github.com/KirkDiggler/rpg-project/issues/1147#issuecomment-7122296542).
- Second Wind's declaration id is `v2.B-E_FF4_AoxcpUkZHlAQQWFnPoVOrjnLkQXLC_slPeA` at `origin/main` and at this head: unchanged — [https://github.com/KirkDiggler/rpg-project/issues/1147](https://github.com/KirkDiggler/rpg-project/issues/1147).
- The smallest hostile creature the site needed: one template (human, 2d8, dagger), one faction behaviour with one `time` row `attack: enemy`, wired on both `time` and `turn` (on `time` alone the guard never attacks), and a monster binding — [https://github.com/KirkDiggler/rpg-project/issues/1147](https://github.com/KirkDiggler/rpg-project/issues/1147).
- New tests: the before/after comparison passed on the unchanged `activate.go`; the went-through-the-door test and the site suite passed once the site and change were in (the site first failed with a guard that never attacked) — [https://github.com/KirkDiggler/rpg-project/issues/1147](https://github.com/KirkDiggler/rpg-project/issues/1147).
- Proof 1 (back through `NewActivation`): the owner is not named on the modifier and the went-through-the-door test fails, as does the roster test — [Amendment 2](https://github.com/KirkDiggler/rpg-project/issues/1147#issuecomment-7122307741).
- Proof 2 (nil cost): neither the pool (still 1) nor the bonus action (still 1) is spent; four tests fail — [Amendment 2](https://github.com/KirkDiggler/rpg-project/issues/1147#issuecomment-7122307741).
- Proof 3 (neutral stance): the scene's first step fails, no fight ("the fight is on and alice is first") — [https://github.com/KirkDiggler/rpg-project/issues/1147](https://github.com/KirkDiggler/rpg-project/issues/1147).
- Existing tests changed: activation_events_test.go, join_long_rest_test.go, rest_test.go (shared pool), world_clock_doors_test.go (row refusal), selector_projection_test.go (classification), the roster test (owner on the modifier) — [https://github.com/KirkDiggler/rpg-project/issues/1147](https://github.com/KirkDiggler/rpg-project/issues/1147).
- Suite at `origin/main` before the pins: ok (59.9s); at the final head: ok (60.7s); `go test -race` of SecondWindWalk|Activat|GuardHailsWalk: ok in 3.8s, the new suite 0.26s; vet and golangci-lint clean — [https://github.com/KirkDiggler/rpg-project/issues/1147](https://github.com/KirkDiggler/rpg-project/issues/1147).

## Mistaken

- A first edit of rest_test.go with a bad replace corrupted the file; caught by `go vet` and restored from git before any commit — [https://github.com/KirkDiggler/rpg-project/issues/1147](https://github.com/KirkDiggler/rpg-project/issues/1147).
- The first golangci-lint run flagged the deprecated `ActionInput.TargetID`; the slice form replaced it — [https://github.com/KirkDiggler/rpg-project/issues/1147](https://github.com/KirkDiggler/rpg-project/issues/1147).
