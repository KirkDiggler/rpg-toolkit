# Run — 2026-11-28 — NPC behaviour 3b-1 a row can activate an ability

Slice [rpg-project#1233](https://github.com/KirkDiggler/rpg-project/issues/1233) of story [rpg-project#1131](https://github.com/KirkDiggler/rpg-project/issues/1131). Base: `rpg-toolkit` `main` at [`5a144ed4`](https://github.com/KirkDiggler/rpg-toolkit/commit/5a144ed4).

One line per entry. Every line ends with a link.

## Shipped

- `Answer.Activate`, `Facts.CanActivate`, the `affords` case, `words()` and composition-row counting, with `activate_test.go` — [rpg-project#1233](https://github.com/KirkDiggler/rpg-project/issues/1233).

## Decided

- The word is carried and judged only by the caller's list (exact match, nothing else gates it); a temperament says nothing about it, so it loads at 100 — [ADR-0051](https://github.com/KirkDiggler/rpg-toolkit/blob/5a144ed4/docs/adr/0051-a-driven-turn-can-activate-an-ability.md).

## Found

- F1-F5 hold at base: tag `mind/behavior/v0.25.0`, `answer.go:124`, `table.go:1091` and `:1216`, `composer.go:139` and `:164`, and build, vet and test pass before any change — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1233#issuecomment-7226943613).
- F6: no existing test pins a row's words list or whole JSON in a way the new field moves; none was edited — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1233#issuecomment-7226943613).
- Every non-test `Dash` hit is accounted for (gains `activate`, or does not and why) — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1233#issuecomment-7226943613).
- Test 5 reaches `checkCompositionRows` through `NewComposer`, with `compositionSpec` and `compositionVocabulary` from `composer_test.go` — [activate_test.go](https://github.com/KirkDiggler/rpg-toolkit/blob/feat/a-row-can-activate-an-ability/mind/behavior/activate_test.go).
- Proof a (no `affords` case): Test 1 failed, "Should be false" on affordable and "should have 1 item(s), but has 2" — [https://github.com/KirkDiggler/rpg-project/issues/1233](https://github.com/KirkDiggler/rpg-project/issues/1233).
- Proof b (no `words()` entry): Test 4 failed, "Expected error with behavior: group cannot be read in chain but got nil" — [https://github.com/KirkDiggler/rpg-project/issues/1233](https://github.com/KirkDiggler/rpg-project/issues/1233).
- Proof c (not counted in `checkCompositionRows`): Test 5 failed, "An error is expected but got nil" — [https://github.com/KirkDiggler/rpg-project/issues/1233](https://github.com/KirkDiggler/rpg-project/issues/1233).
- Proof d (no `omitempty`): Test 6 failed, the plain row contained "Activate"; `TestATableWithoutLoudnessKeepsItsDigest` failed too, its digest moved — [https://github.com/KirkDiggler/rpg-project/issues/1233](https://github.com/KirkDiggler/rpg-project/issues/1233).
- Suite before: `go build`, `go vet`, `go test ./... -count=1` all ok; at the final head: see the PR body — [https://github.com/KirkDiggler/rpg-project/issues/1233](https://github.com/KirkDiggler/rpg-project/issues/1233).

## Mistaken

- none observed — [https://github.com/KirkDiggler/rpg-project/issues/1233](https://github.com/KirkDiggler/rpg-project/issues/1233).
