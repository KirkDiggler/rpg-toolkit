# Run — 2026-11-24 — second wind one way S-resolver the resolver meets the retirement

Slice [rpg-project#1150](https://github.com/KirkDiggler/rpg-project/issues/1150) of story [rpg-project#1131](https://github.com/KirkDiggler/rpg-project/issues/1131). Base: `rpg-toolkit` `main` at [`73622dc8`](https://github.com/KirkDiggler/rpg-toolkit/commit/73622dc8).

## Shipped

- The pin moves `rulebooks/dnd5e` v0.228.0 to v0.229.0; 3 tests moved onto Second Wind's description; 1 comparison test replaced by fixed numbers (7 at level 1, 9 at level 3); 1 new test pins the feature path's refusal — see this slice's PR, opened from branch `test/the-resolver-meets-the-retirement` — [issue](https://github.com/KirkDiggler/rpg-project/issues/1150).

## Decided

- The helper `resolveSecondWindFromItsDescription` lives in `effect_test.go` beside the other description helpers and puts the fighter alone in the fight, as the moved tests had it — [issue Design 3](https://github.com/KirkDiggler/rpg-project/issues/1150).
- The two moved tests keep every assertion; the expected calculation takes the actor's id as `SourceID` on the "Fighter level" modifier, set in the test, so the shared fixture other tests use is untouched — [issue Allowed edits](https://github.com/KirkDiggler/rpg-project/issues/1150).

## Found

- Rests on: F1 (v0.228.0 pinned; `ErrUsedFromItsDescription` 2 hits; `f.(features.Described)` at action_economy.go:421), F2 (activation.go:676), F3 (4 lines at activation_test.go 415, 440, 582 and effect_test.go 144), F4 (effect_test.go:38) all hold — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1150).
- go.mod diff: only the `rulebooks/dnd5e` line moves, v0.228.0 to v0.229.0; go.sum swaps that module's two lines — [issue Design 1](https://github.com/KirkDiggler/rpg-project/issues/1150).
- Baseline at origin/main before the pin: `go build` and `go test ./... -count=1` ok — [issue Steps](https://github.com/KirkDiggler/rpg-project/issues/1150).
- After the pin alone, 3 tests fail (4 with subtests), all with "resolution: ability refused to activate: this ability is used from its description: build the action with Definition and pay its cost at the gate": TestSecondWindActivationCapturesTheCompleteCalculation (activation_test.go:423), TestSecondWindCapturesZeroAppliedWithoutDroppingRollFacts (:448), TestAnEffectAnnouncesTheHealTheFeatureAnnounces fighter_level_1 and fighter_level_3 (effect_test.go:149) — [issue Step 2](https://github.com/KirkDiggler/rpg-project/issues/1150).
- F3: line 582 (TestTheDoorRefusesARollerlessActivation) only builds the machine and expects ErrBadActivation, it passes unchanged; lines 428, 453 are expected values of the moved tests; 734-871 only build values (source refs and names) and never resolve the feature — [issue F3](https://github.com/KirkDiggler/rpg-project/issues/1150).
- Moved tests, assertions changed: the two activation tests changed one assertion each, `SourceID` on the "Fighter level" modifier from empty to `activation-fighter` (the description names its owner); every other assertion is identical. The comparison test (renamed TestAnEffectAnnouncesTheHealForTheFighterLevel) now asserts Requested 7 and 9, the calculation total, and After — [issue Design 4](https://github.com/KirkDiggler/rpg-project/issues/1150).
- Proof 1: expecting Amount 3 instead of 2 in TestSecondWindActivationCapturesTheCompleteCalculation fails on the Amount line, then restored — [issue Step 6](https://github.com/KirkDiggler/rpg-project/issues/1150).
- Proof 2: expecting success from the Design 2 test fails with "Received unexpected error", then restored — [issue Step 6](https://github.com/KirkDiggler/rpg-project/issues/1150).
- Final head: build, vet, `go test ./... -count=1` ok; `go test -race -run 'Activation|Effect|SecondWind|LongRest'` ok in 22s; golangci-lint 0 issues — [issue](https://github.com/KirkDiggler/rpg-project/issues/1150).

## Mistaken

- none observed — [issue](https://github.com/KirkDiggler/rpg-project/issues/1150).
