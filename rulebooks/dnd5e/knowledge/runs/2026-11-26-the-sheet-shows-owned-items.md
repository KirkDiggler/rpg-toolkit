# Run — 2026-11-26 — item story S the sheet shows owned items

Slice [rpg-project#1170](https://github.com/KirkDiggler/rpg-project/issues/1170) of story [rpg-project#850](https://github.com/KirkDiggler/rpg-project/issues/850). Base: `rpg-toolkit` `main` at [`ce7eab72`](https://github.com/KirkDiggler/rpg-toolkit/commit/ce7eab72fb4e4bd7ac3454ffe7ea3a422f55ee7f).

## Shipped

- `EquipmentView.Items` lists each owned item as a row (`Custom` true, `Kind` cloak/amulet/ring/belt/helm/boots, `StatLine` "Second Wind · 1 of 1 charges") and `Slots` lists each worn slot an owned item fits or that holds something, in `WornSlots()` order — [https://github.com/KirkDiggler/rpg-project/issues/1170](https://github.com/KirkDiggler/rpg-project/issues/1170).
- `StatusView.Resources` gains one row per pool of each worn item, keyed `ItemPoolKey`, named "Second Wind · Lieutenant's cloak", outside the class's allowed list — [https://github.com/KirkDiggler/rpg-project/issues/1170](https://github.com/KirkDiggler/rpg-project/issues/1170).
- New exported field: `EquippedItemView.Custom`; new unexported helpers `wornSlotKind`, `itemKindOf`, `ownedItemRows`, `ownedItemStatLine`, `wornSlotDefs`, `wornItemResources`, `itemPoolName` — [https://github.com/KirkDiggler/rpg-project/issues/1170](https://github.com/KirkDiggler/rpg-project/issues/1170).

## Decided

- A ring item's `Kind` is "ring" and both ring slots accept `["ring"]`; one function (`wornSlotKind`) makes the kind for an item and a slot, as the issue asks — [Design 1](https://github.com/KirkDiggler/rpg-project/issues/1170).
- The stat line's charges part reads "<current> of <maximum> <pool name>", pools in name order after the abilities; the issue's one example fixes the shape for the pool named `charges` — [Design 1](https://github.com/KirkDiggler/rpg-project/issues/1170).
- An ability the rulebook cannot describe is shown as the item's name, as the issue's gap list directs — [https://github.com/KirkDiggler/rpg-project/issues/1170](https://github.com/KirkDiggler/rpg-project/issues/1170).

## Found

- Rests on F1 to F5 all hold at `ce7eab72`; F4 prints only lines inside `character/` (equipment_display.go, status_view.go, a comment in equipment_slots.go), so nothing outside `character/` builds or copies either view — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1170#issuecomment-7172884696).
- Test 1 bytes were captured from the draft-made fighter (`wearer(t, classes.Fighter, nil)`) before any edit and are pinned in the test; the only difference is the new `"Custom":false` on each catalogue row, which the test strips and asserts false separately; `StatusView` bytes are identical — [https://github.com/KirkDiggler/rpg-project/issues/1170](https://github.com/KirkDiggler/rpg-project/issues/1170).
- Proof A: listing every worn slot always fails Test 1, Test 2, the ring test and the reload test; proof B: adding a resource row for an unworn item fails Test 2 — [https://github.com/KirkDiggler/rpg-project/issues/1170](https://github.com/KirkDiggler/rpg-project/issues/1170).
- Suite: `go build`, `go vet`, `go test ./... -count=1 -timeout 20m` and `golangci-lint run ./...` (0 issues) pass at the final head; the race run of the view tests passes in about 23 seconds — [https://github.com/KirkDiggler/rpg-project/issues/1170](https://github.com/KirkDiggler/rpg-project/issues/1170).

## Mistaken

- The first mutation run saved its backups under an unset `TMPDIR`, so the second mutation ran on a tree still carrying the first; both mutations were restored by hand and re-run one at a time before the result above was taken — [https://github.com/KirkDiggler/rpg-project/issues/1170](https://github.com/KirkDiggler/rpg-project/issues/1170).
