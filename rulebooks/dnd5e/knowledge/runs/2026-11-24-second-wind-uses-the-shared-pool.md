# Run — 2026-11-24 — one way to pay for an ability S1 second wind uses the shared pool

Slice [rpg-project#1144](https://github.com/KirkDiggler/rpg-project/issues/1144) of story [rpg-project#1131](https://github.com/KirkDiggler/rpg-project/issues/1131). Base: `rpg-toolkit` `main` at [`c5a8e9a1`](https://github.com/KirkDiggler/rpg-toolkit/commit/c5a8e9a1).

One line per entry. Every line ends with a link.

## Shipped

- Second Wind's uses are counted in the character's shared `second_wind` pool (fighter, short rest, sized from the level-1 grant's `uses`); the feature spends from it and reports it; its private counter, `uses`/`max_uses` and rest listener are gone — [commit](https://github.com/KirkDiggler/rpg-toolkit/commit/25ae86c3).
- The shared view reads the pool (`featureResourceInfo` 1 of 1 before a use, 0 of 1 after); `ownerResourceAllowed` lets a fighter hold `second_wind`; the key's comment is corrected — [commit](https://github.com/KirkDiggler/rpg-toolkit/commit/25ae86c3).
- New `character/second_wind_pool_test.go` (nine tests) — [commit](https://github.com/KirkDiggler/rpg-toolkit/commit/25ae86c3).

## Decided

- No saved character is migrated and no pool is added at load: a fighter saved before this slice has no pool and cannot use Second Wind until a level-up creates it full, as the operator ruled 2026-11-24 ("we do not have any long term characters right now") — [issue design](https://github.com/KirkDiggler/rpg-project/issues/1144).
- `resizeClassResources` and `recoverOnRest` needed no edit: both are generic over `buildClassResources` and the pool's reset type, so `advance.go` and `character.go` are untouched — [issue design 1 and 4](https://github.com/KirkDiggler/rpg-project/issues/1144).
- `createSecondWind` accepts and ignores its config; the grant's `uses` is read in `secondWindGrantUses` where the pool is built (a missing or zero `uses` means one, as the factory did) — [issue design 3](https://github.com/KirkDiggler/rpg-project/issues/1144).
- Existing tests that pinned the private counter's own behaviour were removed or re-aimed at the nearest surviving feature (Action Surge): the old-bounds cases for Second Wind in two tests, the four rest-listener tests in `features/second_wind_test.go`, and the attach tests that used Second Wind as the subscribing feature — [issue allowed edits](https://github.com/KirkDiggler/rpg-project/issues/1144).
- The integration mock fighter stands in for the character's rest verb by refilling its own pool; the character-level rest tests prove the real verb — [issue allowed edits](https://github.com/KirkDiggler/rpg-project/issues/1144).

## Found

- Rests on F1 to F9 all hold at `c5a8e9a1` (F4 prints three lines: the call, the comment and the function) — [rests-on comment](https://github.com/KirkDiggler/rpg-project/issues/1144#issuecomment-7121619944).
- Baseline suite before any change: 39 packages ok, none failing; after the change: 39 ok, none failing — [commit](https://github.com/KirkDiggler/rpg-toolkit/commit/25ae86c3).
- A `RestEvent` published straight on the bus refills only features that subscribe; shared pools refill only through `ShortRest`/`LongRest`, so tests that published the event to refill Second Wind had to call the verb — [character.go recoverOnRest](https://github.com/KirkDiggler/rpg-toolkit/blob/c5a8e9a1/rulebooks/dnd5e/character/character.go#L633).
- Before the change the new tests failed as follows: new fighter has the pool, spends the pool, rests restore, save and load, shared view (expected 1 of 1, got 0 of 0), status-view allow rule, and old-blob (still wrote `uses`); the second-use refusal and the Rage/ki neighbours passed beforehand (the private counter refused; neighbours are unchanged by design) — [commit](https://github.com/KirkDiggler/rpg-toolkit/commit/25ae86c3).
- Proofs, each restored after: no Fighter case fails seven pool tests; `Activate` not spending fails the second-use and spend tests; `ownerResourceAllowed` unchanged fails the shared-view and status-view tests — [commit](https://github.com/KirkDiggler/rpg-toolkit/commit/25ae86c3).
- Existing tests changed: about 25 tests and cases in 8 files (fixtures gaining the pool, private-counter reads moved to the pool, rest-listener tests removed, 0 of 0 pins now 1 of 1, one catalog row flipped) — [commit](https://github.com/KirkDiggler/rpg-toolkit/commit/25ae86c3).

## Mistaken

- The first not-spending mutant did not compile (an unused variable) and so reported nothing; it was rewritten and then failed the tests as expected — [commit](https://github.com/KirkDiggler/rpg-toolkit/commit/25ae86c3).
