# Run — 2026-11-28 — named abilities S8a rage is a named ability

Slice [rpg-project#1288](https://github.com/KirkDiggler/rpg-project/issues/1288) of story [rpg-project#1131](https://github.com/KirkDiggler/rpg-project/issues/1131). Base: `rpg-toolkit` `main` at [`e987dd73`](https://github.com/KirkDiggler/rpg-toolkit/commit/e987dd73).

One line per entry. Every line ends with a link.

## Shipped

- `rage` is in the table of named abilities with one value, `damage_bonus` (a whole number of at least 1, or the string "barbarian level" for the class feature) — [rpg-project#1288](https://github.com/KirkDiggler/rpg-project/issues/1288).
- `features.ValidateParameters` holds each value the entry's table names to a whole number within Min and Max, so content cannot write the by-level form — [rpg-project#1288](https://github.com/KirkDiggler/rpg-project/issues/1288).
- `Rage.Definition` asks the entry, as the fighter's two features do; what it returns is unchanged at level 1 and level 20 — [rpg-project#1288](https://github.com/KirkDiggler/rpg-project/issues/1288).

## Decided

- The entry's refusals all start `rage` and name `damage_bonus`; the below-minimum form reads `rage damage_bonus <json> must be at least 1 (a whole number), or "barbarian level"` so `TestAbilityWordsAgreeWithNamedAbilitiesAndTheDecode` (which looks for `must be at least N`) passes unedited — [rpg-project#1288](https://github.com/KirkDiggler/rpg-project/issues/1288).
- The fixed-form description reads "Enter a rage. While raging you add N to the damage of melee weapon hits made with Strength, ..."; the by-level form is the feature's own sentence — [rpg-project#1288](https://github.com/KirkDiggler/rpg-project/issues/1288).
- Tests that used Rage as the unknown ability take `no such ability` / `no_such_ability` / `no-such-ability`, by amendment 1 — [amendment 1](https://github.com/KirkDiggler/rpg-project/issues/1288#issuecomment-7236011660).

## Found

- F3 holds: `ValidateParameters` is asked only by `kits/compile.go:245` and `kits/kit.go:141`; `DescribeFromRef` by `second_wind.go:94`, `action_surge.go:83`, `character/worn_items.go:97`, `monster/granted.go:94` — [Rests on](https://github.com/KirkDiggler/rpg-project/issues/1288#issuecomment-7235978266).
- F4's grep found the list pins but not the tests that use `rage` as the unknown; two failed and one passed having lost its subject — [the gap](https://github.com/KirkDiggler/rpg-project/issues/1288#issuecomment-7236003737).
- `named_abilities_internal_test.go` also pins `Len(words, 2)` after the table is restored (line 100), a fourth pin F4 did not name — [rpg-project#1288](https://github.com/KirkDiggler/rpg-project/issues/1288).
- The count: production files changed are `features/rage_named_ability.go` (new), `features/named_abilities.go`, `features/rage.go`; no giver's code (`character`, `monster`, `kits`, `magicitems`) changed — [rpg-project#1288](https://github.com/KirkDiggler/rpg-project/issues/1288).
- Proofs: (a) table line removed: Tests 1, 2, 3, 4, 5, 6, 7 and the barbarian's own rage suites fail; (b) values-table check skipped: `TestContentWritesANumberForRage` and the kits by-level refusals fail; (c) `describe` drops `in.pools`: `TestRageDescribedWithANumber`, the wearer test and the creature test fail — [rpg-project#1288](https://github.com/KirkDiggler/rpg-project/issues/1288).
- Suite in `rulebooks/dnd5e`: build, vet and `go test ./... -count=1` pass before (`e987dd73`) and at the final head; `golangci-lint` 0 issues — [rpg-project#1288](https://github.com/KirkDiggler/rpg-project/issues/1288).

## Mistaken

- Proof (b) first printed nothing because the change did not compile (an unused variable); a build failure does not match a `--- FAIL` filter, and the proof was re-run with `_ = entry` — [rpg-project#1288](https://github.com/KirkDiggler/rpg-project/issues/1288).
- The first F4 pass missed the tests that use `rage` as the unknown example; the lead amended — [amendment 1](https://github.com/KirkDiggler/rpg-project/issues/1288#issuecomment-7236011660).
