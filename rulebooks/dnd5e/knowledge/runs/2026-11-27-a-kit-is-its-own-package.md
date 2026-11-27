# Run — 2026-11-27 — NPC ability 2a-1 a kit is its own package

Slice [rpg-project#1225](https://github.com/KirkDiggler/rpg-project/issues/1225) of story [rpg-project#1131](https://github.com/KirkDiggler/rpg-project/issues/1131). Base: `rpg-toolkit` `main` at [`b440d37b`](https://github.com/KirkDiggler/rpg-toolkit/commit/b440d37b).

## Shipped

- Package `rulebooks/dnd5e/kits`: `Pool`, `Grant`, `Kit` with `Clone` and `Validate(holder)`, `Compile` of a kit's `charges` and `lets`, and the kit's words `Words` — [261b734e](https://github.com/KirkDiggler/rpg-toolkit/commit/261b734e).
- `magicitems` holds a kit: `Pool`, `Grant` and the word types and constants are aliases of the kit's, `Description` keeps its two fields and gains `Kit()`, and its validation, compile and words delegate — [261b734e](https://github.com/KirkDiggler/rpg-toolkit/commit/261b734e).
- A test that an item's saved shape is its own and not the kit's — [aa16388f](https://github.com/KirkDiggler/rpg-toolkit/commit/aa16388f).

## Decided

- Whether `lets` may be absent or empty stays in `magicitems`, after `kits.Compile` has read `charges` and `lets`, because that keeps the order an item has always refused in and the sentences identical; the kit compiles an absent or empty `lets` to no grants — [rpg-project#1225](https://github.com/KirkDiggler/rpg-project/issues/1225).
- `kits.Compile` does not validate the kit; the item's final "the item is not one the rulebook can run" check stays in `magicitems`, because it names the item by id and its sentence is the item's — [rpg-project#1225](https://github.com/KirkDiggler/rpg-project/issues/1225).
- `kits` keeps its own `validName`, and `magicitems` keeps its own for an item id, rather than one exported helper, because no new name was handed out — [rpg-project#1225](https://github.com/KirkDiggler/rpg-project/issues/1225).
- The refusal sentences keep "the item" wherever they said it today, so a stat block's refusal reads `stat block "gate-lieutenant": ... which the item does not declare`, because the slice holds an item's sentences byte for byte — [rpg-project#1225](https://github.com/KirkDiggler/rpg-project/issues/1225).
- `magicitems.PlaceItem` stays in `magicitems`, so `kits.Words()` is exactly the rows whose place is not `item` — [rpg-project#1225](https://github.com/KirkDiggler/rpg-project/issues/1225).
- Test 7 reads `go list -deps` for the kit package, because it also catches an import that arrives through another package — [kits/kit_test.go](https://github.com/KirkDiggler/rpg-toolkit/blob/aa16388f/rulebooks/dnd5e/kits/kit_test.go).

## Found

- F1 to F4 hold at `b440d37b`; F2 lists the `character` users and, in the separate `resolution` and `session` modules, `long_rest.go`, `long_rest_clone_test.go`, `loot_delivery.go`, `loot_delivery_internal_test.go`, `statuses.go` and `worn_item_row_test.go` — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1225#issuecomment-7206761731).
- F5: `go test ./... -count=1 -timeout 20m` in `rulebooks/dnd5e` passes at `b440d37b` with 40 packages ok, and 41 at the final head (the new `kits`); the Words JSON (2527 bytes, sha256 `820a7e91dc287987b7af2365af4b102a51841d039fc346619603d726cfb075ec`) and ten Validate refusals are byte for byte the same after — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1225#issuecomment-7206761731).
- Proof (a), a kit refusal changed by a word (`from` to `between` in `charges.max`): `TestCompileIsByteForByteWhatItWasBeforeTheTable` fails with `expected: ... a whole number from 1 to 1000` and `actual: ... a whole number between 1 to 1000`, and so do the kit's own refusal tests. A first try on `it must be a whole number` failed nothing, because no guard case writes a non-integer `max`; that sentence is held by no test — [rpg-project#1225](https://github.com/KirkDiggler/rpg-project/issues/1225).
- Proof (b), `Description` embedding `kits.Kit`: this toolchain (`go 1.27.0` in `rulebooks/dnd5e/go.mod`) accepts a promoted field in a composite literal, so `character`'s tests compile and every existing test passes; the embedding changes an item's saved JSON (`grants` is dropped when empty, since the kit's tag is `omitempty`), which no existing test caught. The `kits` test package fails to build (the field shadows `Kit()`) — [gap comment](https://github.com/KirkDiggler/rpg-project/issues/1225#issuecomment-7206841322).
- Proof (c), the `refills` row dropped from the kit's table: the `magicitems` tests `TestCompileTheCloak`, `TestCompileRefills`, `TestCompileRefusals` (three cases), `TestCompileKeepsNoReferenceToTheAuthorsBytes`, `TestCompileUniqueTrueIsUnique`, `TestCompileWithoutUniqueMarshalsNoUniqueKey`, `TestCloneOfUniqueIsUniqueAndIndependent`, `TestCompileIsByteForByteWhatItWasBeforeTheTable`, `TestAWordNotInTheTableIsRefusedAndTheRefusalOffersTheTable`, `TestEveryClosedValueCompilesAndOneOutsideTheTableDoesNot` and `TestWordsAreInTheOrderTheRefusalsOfferThem` fail, and so do `TestAKitAloneCompilesToWhatTheSameWordsGiveAnItem` and `TestEveryRefusalReadsUnderTheHoldersNameWithTheItemsSentence` in `kits` — [261b734e](https://github.com/KirkDiggler/rpg-toolkit/commit/261b734e).

## Mistaken

- The first proof (a) edit changed a sentence no test holds and passed; the edit was restored and a covered sentence used — [rpg-project#1225](https://github.com/KirkDiggler/rpg-project/issues/1225).
- The issue expected proof (b) to fail in `character`'s tests; it does not on this toolchain, and nothing in the existing suite pinned an empty `grants` — [gap comment](https://github.com/KirkDiggler/rpg-project/issues/1225#issuecomment-7206841322).
