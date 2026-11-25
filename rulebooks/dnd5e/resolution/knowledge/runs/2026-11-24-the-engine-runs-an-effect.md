# Run — 2026-11-24 — one way to pay for an ability S3 the engine runs an effect

processed: 2026-11-25T190427Z

Slice [rpg-project#1146](https://github.com/KirkDiggler/rpg-project/issues/1146) of story [rpg-project#1131](https://github.com/KirkDiggler/rpg-project/issues/1131). Base: `rpg-toolkit` `main` at [`88bbe897`](https://github.com/KirkDiggler/rpg-toolkit/commit/88bbe89726c756e1329f0709c371a85e8f016914).

## Shipped

- `NewAction` dispatches a definition whose body is an effect on its user to a new `effectMachine` (`effect.go`); its cost is taken at the door and its heal is delivered through the gateless activation arm a healing spell uses — [rpg-project#1146](https://github.com/KirkDiggler/rpg-project/issues/1146).
- The module pins the rulebook at `v0.228.0`, from `v0.225.0`; `go.mod` moves one requirement line and `go.sum` swaps that module's two lines — [rpg-project#1146](https://github.com/KirkDiggler/rpg-project/issues/1146).

## Decided

- An effect returns an `ActivationOutcome` carrying one healing-applied fact, the shape a feature activation reports, because a cast's `CastOutcome` names a spell and a caster and an effect is neither; no new result type — [issue Design 4](https://github.com/KirkDiggler/rpg-project/issues/1146).
- The actor named as the one target (`TargetID` or a one-element `TargetIDs`) is accepted, any other target is refused with `ErrBadAction`; `normalizeActionTargets` hands the machine a copied slice — [issue Design 3](https://github.com/KirkDiggler/rpg-project/issues/1146).
- The delivery is not copied: the machine builds a `preparedCast` holding the `preparedHealing` and hands it to `NewActivation`'s unexported cast arm, so `healing.go` is untouched — [issue Design 3](https://github.com/KirkDiggler/rpg-project/issues/1146).

## Found

- After the pin alone the test package did not build: six compile errors in `activation_test.go` and `long_rest_test.go` naming `SecondWindData.Uses` and `MaxUses`, which `v0.227.0` removed; no test failed at run time once those compiled — [rpg-project#1146](https://github.com/KirkDiggler/rpg-project/issues/1146).
- A description's fighter-level modifier carries the owner's id as its source id and the feature's own heal leaves it empty; every other part of the two calculations is equal at fighter levels 1 and 3 — [gap comment](https://github.com/KirkDiggler/rpg-project/issues/1146).
- A machine is handed no bus and the sheet has no heal verb (healing is applied by a bus subscriber), so a heal cannot be delivered in `Start`; that proof cannot be written — [gap comment](https://github.com/KirkDiggler/rpg-project/issues/1146).
- The same-turn bonus-action spell case is not reachable from the activation fixture (its fighter holds no spell list); the pin is a cost carrying a spell turn resolving as an effect without a spell classification — [rpg-project#1146](https://github.com/KirkDiggler/rpg-project/issues/1146).
- Rests on F1 to F8 held at `88bbe897`; suite at base `ok` in 1.0s, at head `ok` in 1.0s; `go test -race -run 'Effect|Activation|SecondWind|Cure'` ok in 23s wall; `golangci-lint run ./...` 0 issues — [rests-on comment](https://github.com/KirkDiggler/rpg-project/issues/1146#issuecomment-7121968281).
- Proofs: leaving the effect case out of `NewAction` fails 14 effect tests; a fixed +1 in place of the declaration's modifiers fails "agrees with the old way" at fighter level 3 only — [rpg-project#1146](https://github.com/KirkDiggler/rpg-project/issues/1146).

## Mistaken

- The first agreement assertion compared whole effects and failed on the modifier source id; it now pins that one known difference by name and compares the rest — [gap comment](https://github.com/KirkDiggler/rpg-project/issues/1146).
