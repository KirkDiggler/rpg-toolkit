# Run — 2026-11-26 — item story S1 the item words are published

Slice [rpg-project#1186](https://github.com/KirkDiggler/rpg-project/issues/1186) of story [rpg-project#850](https://github.com/KirkDiggler/rpg-project/issues/850). Base: `rpg-toolkit` `main` at [`a193b5fb`](https://github.com/KirkDiggler/rpg-toolkit/commit/a193b5fb).

## Shipped

- `features.AbilityWords`: one `AbilityWord` per named ability, in `NamedAbilities()` order, with `level` (required, min 1, no upper limit) for second wind — [rpg-project#1186](https://github.com/KirkDiggler/rpg-project/issues/1186).
- `magicitems.Words`: the item's twelve words as one table with place, shape, closed values, bounds and label; each `use` value carries its `With` — [rpg-project#1186](https://github.com/KirkDiggler/rpg-project/issues/1186).
- `magicitems.Compile` reads the table for its allowed words, slots, rests and `charges.max` bounds — [rpg-project#1186](https://github.com/KirkDiggler/rpg-project/issues/1186).

## Decided

- The shared minimum is one constant, `secondWindMinLevel`, read by the decode and by `AbilityWords`; the refusal sentence is unchanged byte for byte — [rpg-project#1186](https://github.com/KirkDiggler/rpg-project/issues/1186).
- `cost.charges` is in the table with no upper limit; the "at most the item's charges" refusal stays in the compiler and the word's doc comment says so, as the issue gave — [rpg-project#1186](https://github.com/KirkDiggler/rpg-project/issues/1186).
- `cost.charges` at least 1 and `name` text checks stay where they were: the issue lists only four things the compiler takes from the table — [rpg-project#1186](https://github.com/KirkDiggler/rpg-project/issues/1186).
- The refusal sentence for `charges.max` now prints the table's bounds, which print the same "1 to 1000" — [rpg-project#1186](https://github.com/KirkDiggler/rpg-project/issues/1186).

## Found

- F1 to F4 hold at `a193b5fb`: `status.Words` at `words.go:122`, the five `c.allowed` calls, four `features` lines, and `features` does not import `magicitems` — [Rests on comment](https://github.com/KirkDiggler/rpg-project/issues/1186#issuecomment-7185051626).
- The full dnd5e suite is 40 packages ok before and at the head, none failing — [rpg-project#1186](https://github.com/KirkDiggler/rpg-project/issues/1186).
- Proof 1, remove the `unique` row: `TestCompileUniqueTrueIsUnique`, `TestCompileWithoutUniqueMarshalsNoUniqueKey`, `TestCompileUniqueMustBeTrueOrFalse`, `TestCompileUnknownWordListsUniqueInOrder`, `TestCloneOfUniqueIsUniqueAndIndependent`, `TestCompileIsByteForByteWhatItWasBeforeTheTable`, `TestWordsAreInTheOrderTheRefusalsOfferThem` fail — [rpg-project#1186](https://github.com/KirkDiggler/rpg-project/issues/1186).
- Proof 2, add slot `gloves` with no sheet slot: `TestEveryClosedValueCompilesAndOneOutsideTheTableDoesNot` fails (the item is refused, as it has no sheet slot) and `TestCompileIsByteForByteWhatItWasBeforeTheTable` fails on "bad slot" and "no slot", whose sentences now offer gloves — [rpg-project#1186](https://github.com/KirkDiggler/rpg-project/issues/1186).
- Proof 3, `AbilityWords` min 0 with the decode unchanged: `TestAbilityWordsAgreeWithNamedAbilitiesAndTheDecode` fails, a value at the reported min is refused — [rpg-project#1186](https://github.com/KirkDiggler/rpg-project/issues/1186).

## Mistaken

- none observed — [rpg-project#1186](https://github.com/KirkDiggler/rpg-project/issues/1186).
