# Run — 2026-11-28 — NPC behaviour 4a a when reads its creature's own hit points

Slice [rpg-project#1244](https://github.com/KirkDiggler/rpg-project/issues/1244) of story [rpg-project#1131](https://github.com/KirkDiggler/rpg-project/issues/1131). Base: `rpg-toolkit` `main` at [`351393dd`](https://github.com/KirkDiggler/rpg-toolkit/commit/351393dd).

One line per entry. Every line ends with a link.

## Shipped

- `When.HitPoints` and `HitPointsWhen`, `Facts.HitPoints` and `Facts.MaxHitPoints`, the `holds` case, `conditions()` and `Validate`, the README rows, and `hit_points_test.go` — [rpg-project#1244](https://github.com/KirkDiggler/rpg-project/issues/1244).

## Decided

- The author writes the number, a whole percent 1 to 100 of the creature's maximum; the module compares in whole numbers (`HitPoints*100 <= AtMost*MaxHitPoints`), no division, no rounding; Kirk, 2026-11-28: "A. I like being explicit and it is simple and easy to read" — [rpg-project#1131](https://github.com/KirkDiggler/rpg-project/issues/1131#issuecomment-7228542408).
- A maximum of zero or less is "not told" and holds nothing; `not` of it holds, as `threat`'s does — [rpg-project#1244](https://github.com/KirkDiggler/rpg-project/issues/1244).
- The verdict records nothing for hit points and `forRow`/`namesThreat` do not gain it, since the numbers are the creature's, in the facts — [rpg-project#1244](https://github.com/KirkDiggler/rpg-project/issues/1244).

## Found

- F1-F4 hold at base: tag `mind/behavior/v0.26.0`, `table.go` type, three functions and field, `CanActivate` at :1106, and build, vet, test pass — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1244#issuecomment-7228576107).
- F5 (a): `table.go:619` and `table_test.go:1116-1117`, the one allowed edit; (b) `dash_test.go:238,377,384` and `group_test.go:355`, unedited; (c) nothing printed — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1244#issuecomment-7228576107).
- F6: one hit, `table.go:1827`, a verdict being built, not a copy of a When or Facts — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1244#issuecomment-7228576107).
- Every non-test `Threat` hit is accounted for (gains, or does not and why) — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1244#issuecomment-7228576107).
- Tests were seen red by compile failure first (`behavior.HitPointsWhen` undefined), then green after the change — [commit](https://github.com/KirkDiggler/rpg-toolkit/commit/5b9c497b).
- Proof a (`<` for `<=`): TestAtOrBelowTheShareAndNotAbove failed, expected true actual false (3 of 10) — [https://github.com/KirkDiggler/rpg-project/issues/1244](https://github.com/KirkDiggler/rpg-project/issues/1244).
- Proof b (no `MaxHitPoints > 0` guard): TestNotToldHoldsNothing failed (two "Should be false") and TestNotIsPlainNegation failed, since 0 of 0 then holds — [https://github.com/KirkDiggler/rpg-project/issues/1244](https://github.com/KirkDiggler/rpg-project/issues/1244).
- Proof c (no `hit_points` in `conditions()`): TestOneConditionPerEntry failed (no error for two conditions) and TestTheNumberIsJudged failed (got "names no condition") — [https://github.com/KirkDiggler/rpg-project/issues/1244](https://github.com/KirkDiggler/rpg-project/issues/1244).
- Proof d (no `omitempty`): the four existing JSON tests failed (TestTheRecordReadsDash, TestTheRecordReadsThreat on two lines, TestALiteralNameIsTheStringItWas) and the new record test failed — [https://github.com/KirkDiggler/rpg-project/issues/1244](https://github.com/KirkDiggler/rpg-project/issues/1244).
- Suite: build, vet and `go test ./... -count=1` ok before and at the final head — [https://github.com/KirkDiggler/rpg-project/issues/1244](https://github.com/KirkDiggler/rpg-project/issues/1244).

## Mistaken

- none observed — [https://github.com/KirkDiggler/rpg-project/issues/1244](https://github.com/KirkDiggler/rpg-project/issues/1244).
