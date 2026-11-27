# Run — 2026-11-27 — NPC ability 2a-2 a lets entry carries its own uses

Slice [rpg-project#1226](https://github.com/KirkDiggler/rpg-project/issues/1226) of story [rpg-project#1131](https://github.com/KirkDiggler/rpg-project/issues/1131). Base: `rpg-toolkit` `main` at [`649eec7d`](https://github.com/KirkDiggler/rpg-toolkit/commit/649eec7d).

One line per entry. Every line ends with a link.

## Shipped

- A `lets` entry can write `uses: { max, refills }`: its own pool, named by the grant, and a grant paying one use from it; three new words (`lets entry uses`, `uses max`, `uses refills`) published by `kits.Words` and `magicitems.Words` — [commit 7e33cf3e](https://github.com/KirkDiggler/rpg-toolkit/commit/7e33cf3e).
- The sheet played with it: an item with no `charges` and one entry's own use grants, pays at the gate, refuses a second use, gets it back on its rests, and saves `left` for the entry's pool — [commit 7e33cf3e](https://github.com/KirkDiggler/rpg-toolkit/commit/7e33cf3e).

## Decided

- An entry's own pool is named by its grant (`second-wind`), the one name `lets` already files it under; an entry whose pool would be `charges` is refused so the shared pool is never shadowed — [issue Design 2](https://github.com/KirkDiggler/rpg-project/issues/1226).
- `charges` and `uses` are read by one function, `compiler.pool`; the `charges` sentences are byte for byte as they were, the `uses` sentences carry `lets[n]` and say `uses`, `uses.max`, `uses.refills`, `uses.refills.on` — [issue Design 2](https://github.com/KirkDiggler/rpg-project/issues/1226).
- `cost` with `uses` on one entry is refused: "lets[0] has both `cost` and `uses`; a use is paid from the item's charges or from the entry's own uses, not both" — [issue Design 2](https://github.com/KirkDiggler/rpg-project/issues/1226).

## Found

- Facts: F1 holds; F2 holds; F3 printed `kits/compile_test.go:87` and `magicitems/words_test.go:62`; F4 holds, `itemDefinition` has no special case for `charges`; F5 the suite passed at `649eec7d` — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1226#issuecomment-7206959599).
- The sheet shows an entry's own pool as "Second Wind · Wind cloak" in the resource rows (same as `charges`) and as "Second Wind · 1 of 1 second-wind" in the owned-item stat line, the raw pool name where a shared pool reads "1 of 1 charges"; not changed here — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1226#issuecomment-7206959599).
- Two existing tests pin the whole published word list, not only the lets entry's: `TestTheItemsWordsStartWithTheItemsOwnFiveThenTheKits` (kits/words_test.go) and `TestWordsAreInTheOrderTheRefusalsOfferThem` (magicitems/words_test.go); each got the three new rows in its literal list and nothing else — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1226#issuecomment-7206959599).
- A long rest also brings back a pool that names only `short rest` (`restSatisfies`, as for `charges`), so the issue's "a long rest does too only if the entry names it" holds only for a pool that names `long rest` without `short rest`: a short rest never fills a long-rest pool — [worn_items.go restoreItemPools](https://github.com/KirkDiggler/rpg-toolkit/blob/649eec7d/rulebooks/dnd5e/character/worn_items.go#L193).
- Only one ability is named today and an ability may appear once, so two entries with their own counts cannot be written; Test 3 is the shared `charges` beside an entry's own `uses` in one kit — [issue Test 3](https://github.com/KirkDiggler/rpg-project/issues/1226).
- The guard that refuses an own pool named `charges` cannot be reached with any ability named today; proof (a) reaches it and every own-uses test fails with its sentence — [commit 7e33cf3e](https://github.com/KirkDiggler/rpg-toolkit/commit/7e33cf3e).
- Proofs, each run on the committed work and restored: (a) own pool named `charges`: the entry/kit tests (Tests 2, 3, 4's uses rows) and the three sheet tests fail with the guard's refusal; (b) accept both `cost` and `uses`: only `uses and cost` in the refusal table fails; (c) drop the `uses refills` row: four refusal rows (`unknown uses word`... `uses rest twice`), both order tests, the words test, the compile tests and the three sheet tests fail — [commit 7e33cf3e](https://github.com/KirkDiggler/rpg-toolkit/commit/7e33cf3e).
- Suite: `go test ./... -count=1 -timeout 20m` in `rulebooks/dnd5e` passes before (at `649eec7d`) and at the final head — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1226#issuecomment-7206959599).

## Mistaken

- The issue's rest sentence was taken as written and the sheet test failed on it; the test now states what `restSatisfies` does — [commit 7e33cf3e](https://github.com/KirkDiggler/rpg-toolkit/commit/7e33cf3e).
