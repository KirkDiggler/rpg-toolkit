# Run — 2026-11-28 — Rage 7a an effect may put a status on its user

Slice [rpg-project#1277](https://github.com/KirkDiggler/rpg-project/issues/1277) of story [rpg-project#1131](https://github.com/KirkDiggler/rpg-project/issues/1131). Base: `rpg-toolkit` `main` at [`03dd9960`](https://github.com/KirkDiggler/rpg-toolkit/commit/03dd9960).

## Shipped

- `EffectProfile.Condition *ConditionApplication` (`json:"condition,omitempty"`): an effect is a heal or a named status on its user, refused when it has neither or both, a save, or a counterpart; `Clone` deep-copies it; nothing runs it — [rpg-project#1277](https://github.com/KirkDiggler/rpg-project/issues/1277).

## Decided

- A save or counterpart on the status is refused before the declaration's own validation, so its sentence is the one a reader sees even when the gate is also incomplete — [rpg-project#1277](https://github.com/KirkDiggler/rpg-project/issues/1277).
- The one allowed test edit: `TestAnEffectThatDoesNothingIsRefused` expected `healing`, now `exactly one thing: a heal or a status, and it has neither`, because the old sentence is replaced — [rpg-project#1277](https://github.com/KirkDiggler/rpg-project/issues/1277).

## Found

- F3: `git grep "healing is absent"` printed one line, the definition at `effect.go:38`; the only test matching by fragment is `TestAnEffectThatDoesNothingIsRefused` (`effect_test.go:78`), asserting the error contains `healing` — [rpg-project#1277](https://github.com/KirkDiggler/rpg-project/issues/1277).
- F4 printed seven non-test lines: `definition.go:34` (field), `effect.go:19,22,33,48` (the type and its methods), `features/second_wind_named_ability.go:75` (builds a heal), `session/selector_projection.go:46` (holder, same tag); no hand-written copy of the fields — [rpg-project#1277](https://github.com/KirkDiggler/rpg-project/issues/1277).
- `ref` reads in JSON as a string, `"dnd5e:conditions:raging"`; this package's tests use `encoding/json` — [rpg-project#1277](https://github.com/KirkDiggler/rpg-project/issues/1277).
- A heal-only effect's bytes, pinned: `{"to":"self","healing":{"Dice":"1d10","Modifiers":[{"Source":{"Ref":"dnd5e:classes:fighter","Name":"Fighter","Label":"Fighter level","SourceID":"hero"},"Amount":3}]}}` — [rpg-project#1277](https://github.com/KirkDiggler/rpg-project/issues/1277).
- Proofs: PROOFS — [rpg-project#1277](https://github.com/KirkDiggler/rpg-project/issues/1277).
- Suite: BEFORE before the change and AFTER at the final head — [rpg-project#1277](https://github.com/KirkDiggler/rpg-project/issues/1277).

## Mistaken

- The first `Validate` ran the declaration's own validation before the save rule, so a gate with no recurrence hid the save sentence; the test caught it and the order was swapped — [rpg-project#1277](https://github.com/KirkDiggler/rpg-project/issues/1277).
