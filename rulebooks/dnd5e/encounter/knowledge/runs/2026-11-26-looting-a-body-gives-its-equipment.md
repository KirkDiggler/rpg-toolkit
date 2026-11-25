# Run — 2026-11-26 — item story S3 looting a body gives its equipment

Slice [rpg-project#1167](https://github.com/KirkDiggler/rpg-project/issues/1167) of story [rpg-project#850](https://github.com/KirkDiggler/rpg-project/issues/850). Base: `rpg-toolkit` `main` at [`2b1fd3aa`](https://github.com/KirkDiggler/rpg-toolkit/commit/2b1fd3aa).

## Shipped

- Looting a downed creature as a player takes its equipped custom items off the body once, writes one `taken` beat each to the looter alone, and reports each in `LootOutput.Items`; `ItemGrant.Name` is copied out by the site compiler; the read-back says looting the body gives the items — [slice](https://github.com/KirkDiggler/rpg-project/issues/1167).

## Decided

- The beat order of a loot is `looted`, then what the intel reveals, then `taken` (after the holdings transfer, before the presence sweep and the world's price), so a loot's consequences read in the order they happen — [Design 3 and Test 8](https://github.com/KirkDiggler/rpg-project/issues/1167).
- Items leave the body in item id order, whatever order the author listed them, and the `taken` beats follow that order — [Design 3 and Test 6](https://github.com/KirkDiggler/rpg-project/issues/1167).
- `taken` carries tag `loot`, the tag of the verb that caused it; its audience is the explicit one-element slice `[looter]`, as `appendRevealBeat` does — [F3 answer](https://github.com/KirkDiggler/rpg-project/issues/1167#issuecomment-7172391314).
- The prop's grant carries the name too, since one function (`itemGrantOf`) makes both kinds of grant — [gap 3](https://github.com/KirkDiggler/rpg-project/issues/1167#issuecomment-7172499914).

## Found

- F3: `revealConcealmentTo` scopes its beat through `appendRevealBeat(recipient, ...)`, which appends with `Audience: []MemberID{recipient}`; `audienceFor` is not consulted — [comment](https://github.com/KirkDiggler/rpg-project/issues/1167#issuecomment-7172391314).
- F6: beats are declared only in the const block of `beats.go`; no table, switch or test enumerates them, so `taken` joined that block and nothing else — [comment](https://github.com/KirkDiggler/rpg-project/issues/1167#issuecomment-7172391314).
- The castle content hash did not move: `e3b4252104424f014d2df14d2916a6998c3421d27d0131db4875c8a0a2585312` before and after, because the castle declares no custom items and `Name` omits when empty. F7 expected a move; none happened and the pin is untouched — [F7](https://github.com/KirkDiggler/rpg-project/issues/1167).
- Existing tests edited: `custom_items_test.go` TestCustomItems_NothingInARunUsesThem (it looted an equipped body and compared the answer, beats and bytes; the loot half is removed, the hold half kept, and the loot is now tested in `looting_equipment_test.go`), and its saved-bytes line (gains `"name"`); `dungeonspec/custom_items_test.go` (the prop grant gains the name); `dungeonspec/readback/custom_items_test.go` (the no-loot words rule now covers the item and prop statements only) — [gaps](https://github.com/KirkDiggler/rpg-project/issues/1167#issuecomment-7172498733).
- Proofs: giving `taken` the audience of `looted` fails TestOnlyTheLooterIsToldWhatTheyTook and TestTheLootedBeatIsTheSameWithGearOrWithout; leaving the equipment on the body fails TestASecondLootGivesNothing, TestLootingTakesTheEquipmentOffTheBodyAndTellsTheHost and TestTwoItemsComeInIDOrder — [slice](https://github.com/KirkDiggler/rpg-project/issues/1167).
- Suite: the whole-module `go test ./... -count=1` passes at the final head; `go test -race . -run 'Loot|Holdings|CustomItem|Taken'` passes in 49s; `golangci-lint run ./...` reports 0 issues. A run before the code (tests only) failed exactly the new tests — [slice](https://github.com/KirkDiggler/rpg-project/issues/1167).

## Mistaken

- Three existing tests the issue's allowed edits did not name needed an edit (a saved-bytes string, a prop grant value, the read-back no-loot rule); each was reported as a non-blocking gap before the edit — [gap 1](https://github.com/KirkDiggler/rpg-project/issues/1167#issuecomment-7172498733).
