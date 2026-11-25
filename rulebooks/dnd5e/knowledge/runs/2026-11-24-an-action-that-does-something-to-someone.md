# Run — 2026-11-24 — one way to pay for an ability S2 an action that does something to someone

processed: 2026-11-25T190427Z

Slice [rpg-project#1145](https://github.com/KirkDiggler/rpg-project/issues/1145) of story [rpg-project#1131](https://github.com/KirkDiggler/rpg-project/issues/1131). Base: `rpg-toolkit` `main` at [`68459768`](https://github.com/KirkDiggler/rpg-toolkit/commit/684597682741eb799b2e7e18e2e3825244165a83).

## Shipped

- `actions.EffectProfile` (`to`, `healing`) and `Definition.Effect` as a fourth body, counted, validated and cloned like the others — [rpg-project#1145](https://github.com/KirkDiggler/rpg-project/issues/1145).
- `SecondWind.Definition(owner)` and the `features.Described` interface — [rpg-project#1145](https://github.com/KirkDiggler/rpg-project/issues/1145).
- `Character.AbilityDefinition(ref)` answering the compiled description, or false and no error — [rpg-project#1145](https://github.com/KirkDiggler/rpg-project/issues/1145).

## Decided

- `Describe` and `ResolveSequence` are unchanged: an effect answers prose and no facts, and a sequence step may name an effect as it may name a cast, because the issue named neither file as needing an edit once tests showed both already behave — [rpg-project#1145](https://github.com/KirkDiggler/rpg-project/issues/1145).
- The fighter level modifier names the `fighter` class ref, the name "Fighter", the label "Fighter level" and the owner's id, the same source `Activate` publishes — [rpg-project#1145](https://github.com/KirkDiggler/rpg-project/issues/1145).
- The description carries Second Wind's own `Description()` prose — [rpg-project#1145](https://github.com/KirkDiggler/rpg-project/issues/1145).

## Found

- Rests on F1 to F8 all hold at `68459768` — [comment](https://github.com/KirkDiggler/rpg-project/issues/1145#issuecomment-7121820729).
- Before the change the new tests failed to compile: `EffectProfile`, `EffectSelf`, `Definition.Effect`, `SecondWind.Definition`, `features.Described` and `Character.AbilityDefinition` were undefined — [rpg-project#1145](https://github.com/KirkDiggler/rpg-project/issues/1145).
- Proof one: letting `Definition.Validate` accept two bodies fails `TestAnEffectAndAnAttackAreRefused` — [rpg-project#1145](https://github.com/KirkDiggler/rpg-project/issues/1145).
- Proof two: leaving the pool out of the cost fails `TestALevelOneFightersDescription` — [rpg-project#1145](https://github.com/KirkDiggler/rpg-project/issues/1145).
- Proof three: a fixed +1 fails `TestTheModifierIsTheFighterLevel` — [rpg-project#1145](https://github.com/KirkDiggler/rpg-project/issues/1145).
- `Describe` of Cure Wounds states `Facts.Cast.Healing`; `BaseFacts` has no place for an effect's healing, so an effect states none — [gap comment](https://github.com/KirkDiggler/rpg-project/issues/1145#issuecomment-7121882878).
- `ResolveSequence` refuses only a step naming a sequence or an absent action; an effect component resolves as a clone — [rpg-project#1145](https://github.com/KirkDiggler/rpg-project/issues/1145).
- No other non-test file in the root module switches on the body; the readers of `Cast` and `Sequence` outside it are in the nested modules and were not touched — [rpg-project#1145](https://github.com/KirkDiggler/rpg-project/issues/1145).
- Suite before: `go test ./... -count=1` all ok; at the final head: all ok, `go vet` clean, `golangci-lint` 0 issues — [rpg-project#1145](https://github.com/KirkDiggler/rpg-project/issues/1145).
- Existing tests changed: none — [rpg-project#1145](https://github.com/KirkDiggler/rpg-project/issues/1145).

## Mistaken

- none observed — [rpg-project#1145](https://github.com/KirkDiggler/rpg-project/issues/1145).
