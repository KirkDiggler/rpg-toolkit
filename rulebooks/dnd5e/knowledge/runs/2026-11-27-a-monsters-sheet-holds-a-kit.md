# Run — 2026-11-27 — NPC ability S2b a monster's sheet holds a kit

Slice [rpg-project#1227](https://github.com/KirkDiggler/rpg-project/issues/1227) of story [rpg-project#1131](https://github.com/KirkDiggler/rpg-project/issues/1131). Base: `rpg-toolkit` `main` at [`d9a8cc1b`](https://github.com/KirkDiggler/rpg-toolkit/commit/d9a8cc1b).

## Shipped

- `monster.Data` gains `Kit` and `Left`, both left out when empty; the live sheet holds copies and `Load` refuses a bad saved sheet naming the stat block — [commit](https://github.com/KirkDiggler/rpg-toolkit/commit/54898ff2).
- `monster.PoolKey`, and `PoolLeft` and `SpendPool` answering for a creature's own pools; a turn start leaves the counts alone and nothing refills them — [rpg-project#1227](https://github.com/KirkDiggler/rpg-project/issues/1227).
- `Monster.GrantedAbility` and `Grants()` in `granted.go`, the creature as the giver of the described ability — [rpg-project#1227](https://github.com/KirkDiggler/rpg-project/issues/1227).
- `Template.Lets`, its `Merge` rule, `FromTemplate` compiling it with `kits.Compile` and `CheckLets` judging it with nothing built — [rpg-project#1227](https://github.com/KirkDiggler/rpg-project/issues/1227).

## Decided

- `PoolKey` is `"kit:" + pool`, compared whole and never parsed back, as `character.ItemPoolKey` is — [rpg-project#1227](https://github.com/KirkDiggler/rpg-project/issues/1227).
- An unstated `Lets` (nil) is the base's and a stated one, an empty list too, replaces it whole, as `Actions` does — [rpg-project#1227](https://github.com/KirkDiggler/rpg-project/issues/1227).
- A `lets` that compiles to no pool and no grant builds a creature with no kit, so an empty list saves nothing — [rpg-project#1227](https://github.com/KirkDiggler/rpg-project/issues/1227).
- The kit's sentences are returned as the refusal text unchanged (code InvalidArgument), so `FromTemplate` and `CheckLets` say the same sentence starting `stat block "<id>"` — [rpg-project#1227](https://github.com/KirkDiggler/rpg-project/issues/1227).
- The giver's ref is a copy of the sheet's ref, so a caller changing the description cannot reach the sheet — [rpg-project#1227](https://github.com/KirkDiggler/rpg-project/issues/1227).

## Found

- F1 to F5 hold; the `Holder` type is in `kits/kit.go`, not `compile.go`, so only `CompileInput` printed for that grep — [comment](https://github.com/KirkDiggler/rpg-project/issues/1227#issuecomment-7207094753).
- F2: no import either way between `monster` and `kits` or `features`, and adding them made no cycle (`go build ./...` passes) — [comment](https://github.com/KirkDiggler/rpg-project/issues/1227#issuecomment-7207094753).
- F6: the grep over rpg-toolkit printed one hit, `rulebooks/dnd5e/resolution/depart.go:217 cloneMonsterData`, which copies through its persisted form so it carries both new fields; the same grep over rpg-api `origin/dev` `internal` printed nothing; no new hit — [comment](https://github.com/KirkDiggler/rpg-project/issues/1227#issuecomment-7207094753).
- F7: `go test ./... -count=1` in `rulebooks/dnd5e` passed before the change, 41 packages ok; ToData of the human base with a mace is 689 bytes of JSON — [comment](https://github.com/KirkDiggler/rpg-project/issues/1227#issuecomment-7207094753).
- Test 2's saved kit: `{"pools":{"second-wind":{"maximum":1,"restore_on":["short_rest","long_rest"]}},"grants":{"second-wind":{"ability":{"module":"dnd5e","type":"features","id":"second_wind"},"parameters":{"level":1},"pool":"second-wind","uses":1}}}` and `"left":{"second-wind":1}` — [rpg-project#1227](https://github.com/KirkDiggler/rpg-project/issues/1227).
- Test 3's description text is "Regain 1d10 hit points plus 1"; the cost is one bonus action and one of `kit:second-wind`; the healing modifier is 1 with label and name "Human" and source id "g-1" — [rpg-project#1227](https://github.com/KirkDiggler/rpg-project/issues/1227).
- Proof (a), `omitempty` dropped from `Kit`, failed TestACreatureWithNoKitSavesWhatItSaved and the older TestAMonsterThatSpentNothingSavesNoAccount — [rpg-project#1227](https://github.com/KirkDiggler/rpg-project/issues/1227).
- Proof (b), counts reset at turn start, failed TestOnePaymentTakesTheSlotAndTheUseAllOrNone and TestATurnStartGivesTheTurnBackAndLeavesTheCountAlone — [rpg-project#1227](https://github.com/KirkDiggler/rpg-project/issues/1227).
- Proof (c), `ToData` handing out the live kit, failed TestSaveAndLoadCarryTheKitAndShareNothing — [rpg-project#1227](https://github.com/KirkDiggler/rpg-project/issues/1227).
- Proof (d), `PoolLeft` answering the count for any key, failed TestAKitAnswersOnlyForItsOwnPools; neither Test 10 nor Test 4 failed, because a creature with no kit has no count to answer with and Test 4 asks one key — [rpg-project#1227](https://github.com/KirkDiggler/rpg-project/issues/1227).
- Suite before: 41 packages ok; at the final head: 41 packages ok, `go build`, `go vet` and `golangci-lint run` clean — [rpg-project#1227](https://github.com/KirkDiggler/rpg-project/issues/1227).

## Mistaken

- The implementation was written before the tests, so no test was seen failing for the stated reason first; the four proofs stand in for that — [rpg-project#1227](https://github.com/KirkDiggler/rpg-project/issues/1227).
- The first lint run flagged a `return nil, nil` in the kit helper (nilnil); it now returns a named result — [rpg-project#1227](https://github.com/KirkDiggler/rpg-project/issues/1227).
