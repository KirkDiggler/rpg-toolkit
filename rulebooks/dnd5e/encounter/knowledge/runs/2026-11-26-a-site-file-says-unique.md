# Run — 2026-11-26 — item story a site file says unique

Slice [rpg-project#1181](https://github.com/KirkDiggler/rpg-project/issues/1181) of story [rpg-project#850](https://github.com/KirkDiggler/rpg-project/issues/850). Base: `rpg-toolkit` `main` at [`898a28ec`](https://github.com/KirkDiggler/rpg-toolkit/commit/898a28ec).

## Shipped

- A custom item may say `unique: true` or `false`; any other value is refused at `customItems.<id>.unique`; the block is carried as written; the read-back says at most one or any number — [slice](https://github.com/KirkDiggler/rpg-project/issues/1181).

## Decided

- `unique` sits third in `itemWords`: name, slot, unique, charges, lets — [Design 1](https://github.com/KirkDiggler/rpg-project/issues/1181).
- `unique: yes` is refused: this module's YAML reader (yaml.v3) reads it as the string "yes", and a test pins that — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1181#issuecomment-7182759330).
- `TestCustomItemsAreSaid` checks the item's statement against take, taken and gives only, and the prop's against all four, because an item's statement says what looting does with it and nothing takes a prop yet — [Amendment 1](https://github.com/KirkDiggler/rpg-project/issues/1181).

## Found

- F3 printed one line: `custom_items_test.go:94`, the refusal list pin, changed to the new list; no test pins the whole item statement — [comment](https://github.com/KirkDiggler/rpg-project/issues/1181#issuecomment-7182759330).
- F4: `TestEveryKeyIsClassified` is a method at `readback/notyet_internal_test.go:118`; it walks struct fields, never an item's inner words, and passes unchanged — [comment](https://github.com/KirkDiggler/rpg-project/issues/1181#issuecomment-7182759330).
- The suite was green before at `898a28ec` and at the final head; `golangci-lint` 0 issues; no picture, golden or hash pin moved — [slice](https://github.com/KirkDiggler/rpg-project/issues/1181).
- Proofs: taking `unique` out of `itemWords` fails TestDungeonspec_UniqueLoadsAndIsCarriedAsWritten; the "any number" sentence always fails TestAnItemStatementSaysUniqueOrAnyNumber — [slice](https://github.com/KirkDiggler/rpg-project/issues/1181).

## Mistaken

- The mandated sentences contain "loot", which `TestCustomItemsAreSaid` banned; the issue had not listed that test, so it was reported as a gap and ruled — [gap](https://github.com/KirkDiggler/rpg-project/issues/1181#issuecomment-7182759709).
