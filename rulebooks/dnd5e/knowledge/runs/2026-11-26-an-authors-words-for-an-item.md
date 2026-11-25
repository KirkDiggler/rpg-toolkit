# Run — 2026-11-26 — item story S1 an author's words for an item

Slice [rpg-project#1165](https://github.com/KirkDiggler/rpg-project/issues/1165) of story [rpg-project#850](https://github.com/KirkDiggler/rpg-project/issues/850). Base: `rpg-toolkit` `main` at [`3e03721e`](https://github.com/KirkDiggler/rpg-toolkit/commit/3e03721e).

## Shipped

- `magicitems.Compile` (`CompileInput`, `CompileOutput`) reads an item as an author writes it and returns the `Description` the engine runs — [issue](https://github.com/KirkDiggler/rpg-project/issues/1165).
- `features.NamedAbility` and `features.NamedAbilities` give a rulebook ability by its plain name; one name exists, `second wind` — [issue](https://github.com/KirkDiggler/rpg-project/issues/1165).
- A test in `character/` holds the slot words and `WornSlots()` together — [issue](https://github.com/KirkDiggler/rpg-project/issues/1165).

## Decided

- A whole number is read from its digits (`1` passes, `1.5` and `1e0` are refused) — [issue gap hint](https://github.com/KirkDiggler/rpg-project/issues/1165).
- A `with` the ability refuses is reported by `Compile` itself, naming the item, the entry and the ability's own sentence, before `Description.Validate` runs — [Design 2](https://github.com/KirkDiggler/rpg-project/issues/1165).
- An entry with no `with` is refused for Second Wind, which requires values — [Design 2](https://github.com/KirkDiggler/rpg-project/issues/1165).

## Found

- Rests on F1 to F5 hold at `3e03721e`; the issue's first comment lists them — [comment](https://github.com/KirkDiggler/rpg-project/issues/1165#issuecomment-7171718050).
- Prove-the-tests-bite: accepting unknown top words fails 6 refusal subtests; mapping `ring` to one slot fails `TestCompileSlots/ring` and `TestAuthoredSlotWordsAreTheWornSlots` — [issue](https://github.com/KirkDiggler/rpg-project/issues/1165).
- `go test ./... -count=1` in `rulebooks/dnd5e` passes at the final head; `go test -race ./magicitems/...` takes 7.5s; `golangci-lint run ./...` reports 0 issues — [issue](https://github.com/KirkDiggler/rpg-project/issues/1165).

## Mistaken

- none observed — [issue](https://github.com/KirkDiggler/rpg-project/issues/1165).
