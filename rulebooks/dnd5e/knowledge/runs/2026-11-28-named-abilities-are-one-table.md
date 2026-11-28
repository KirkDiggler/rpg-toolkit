# Run — 2026-11-28 — second ability S5a named abilities are one table

Slice [rpg-project#1256](https://github.com/KirkDiggler/rpg-project/issues/1256) of story [rpg-project#1131](https://github.com/KirkDiggler/rpg-project/issues/1131). Base: `rpg-toolkit` `main` at [`8517aac9`](https://github.com/KirkDiggler/rpg-toolkit/commit/8517aac9).

## Shipped

- A named ability is one `namedAbility` entry in one ordered table (`features/named_abilities.go`); `NamedAbility`, `NamedAbilities`, `ValidateParameters`, `DescribeFromRef` and `AbilityWords` read it, and Second Wind is its one entry — [https://github.com/KirkDiggler/rpg-project/issues/1256](https://github.com/KirkDiggler/rpg-project/issues/1256).
- Five tests in `features/named_abilities_internal_test.go`: a second entry found by all five, refusals named by word, the order of refusals, Second Wind's description written out, a well-formed table — [https://github.com/KirkDiggler/rpg-project/issues/1256](https://github.com/KirkDiggler/rpg-project/issues/1256).

## Decided

- The table is an ordered slice of entry pointers with lookups by word and by ref that scan it, and `add` returns the function that removes the entry, which is how a test lends it a second entry; at one entry a scan is the whole cost — [slice design 2](https://github.com/KirkDiggler/rpg-project/issues/1256).
- `describeInput` holds the raw parameters, the giver and the pools already judged and copied (nil when none); the entry decodes its own values again inside `describe` rather than being handed a typed value, so the shared code never names an ability's value type — [slice design 1](https://github.com/KirkDiggler/rpg-project/issues/1256).
- The shared order is the values (`check`), then the giver, then the pools, which is the order of the old `DescribeFromRef`; `checkedEntry` also serves `ValidateParameters` — [slice design 3](https://github.com/KirkDiggler/rpg-project/issues/1256).
- The shared giver and pool refusals take the entry's word; for Second Wind the word is "second wind", so every message is byte for byte what it was — [slice design 4](https://github.com/KirkDiggler/rpg-project/issues/1256).
- Where an ability is named: no README or package doc in `features` describes it, so the three lines on adding an entry sit on the doc comment of `namedAbility` — [slice files](https://github.com/KirkDiggler/rpg-project/issues/1256).
- Every "second wind" literal outside tests is accounted for: the entry's own, now the entry's word, or left as the fighter's class feature — [F6 comment](https://github.com/KirkDiggler/rpg-project/issues/1256#issuecomment-7230695836).

## Found

- F1 prints 12 lines: `character` 1, `kits` 6, `monster` 3, `session` 2 — [F1](https://github.com/KirkDiggler/rpg-project/issues/1256#issuecomment-7230695836).
- `examples/items` is its own module pinning `rulebooks/dnd5e v0.215.0` with no `replace`: it does not see this change — [F5](https://github.com/KirkDiggler/rpg-project/issues/1256#issuecomment-7230695836).
- A `refactor` title is a patch bump in `auto-tag-modules.yml`; the nearest earlier `refactor` commit, `37c2460a`, cut `rulebooks/dnd5e/resolution/v0.74.3` — [tag](https://github.com/KirkDiggler/rpg-project/issues/1256#issuecomment-7230695836).
- Suite at `8517aac9` and at the final head: `go build`, `go vet`, `go test ./... -count=1` all pass; `golangci-lint run ./...` reports 0 issues at the head — [https://github.com/KirkDiggler/rpg-project/issues/1256](https://github.com/KirkDiggler/rpg-project/issues/1256).
- The proofs, each made on the working tree after the commit and restored: (a) `AbilityWords` reading a hard-coded Second Wind failed Test 1 ("should have 2 item(s), but has 1"); (b) the giver's Ref refusal said the literal "second wind" failed Test 2 (expected "test surge giver requires a Ref"); (c) the giver checked before the values failed Test 3 (expected "second wind level 0 must be at least 1", got "second wind giver requires a Ref"); (d) die changed to `1d8` failed Test 4 and the existing `TestSecondWindDefinitionSuite/TestALevelOneFightersDescription`, `TestSecondWindNamedAbilitySuite/TestAFightersDescriptionIsUnchangedByteForByte` and `TestSecondWindNamedAbilitySuite/TestARingAsksForLevelOne` — [https://github.com/KirkDiggler/rpg-project/issues/1256](https://github.com/KirkDiggler/rpg-project/issues/1256).

## Mistaken

- The implementation was written before the new tests, against the order the slice asked; the tests were not seen failing before the change, and the four proofs after the commit are the evidence that each bites — [https://github.com/KirkDiggler/rpg-project/issues/1256](https://github.com/KirkDiggler/rpg-project/issues/1256).
