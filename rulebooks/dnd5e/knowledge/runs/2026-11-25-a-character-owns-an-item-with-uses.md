# Run — 2026-11-25 — item story S1 a character owns an item with uses

Slice [rpg-project#1162](https://github.com/KirkDiggler/rpg-project/issues/1162) of story [rpg-project#850](https://github.com/KirkDiggler/rpg-project/issues/850). Base: `rpg-toolkit` `main` at [`28b42774`](https://github.com/KirkDiggler/rpg-toolkit/commit/28b42774344eba7bf501fbcd9fe1a60db784694c).

## Shipped

- New leaf package `rulebooks/dnd5e/magicitems`: `Description`, `Pool`, `Grant`, `Data`, `New`, `Description.Validate`, `Data.Validate`, `Description.Clone`, `Data.Clone` — [rpg-project#1162](https://github.com/KirkDiggler/rpg-project/issues/1162).
- `character.Data.Items`, `character.AddItem`, `character.RemoveItem`, `character.WornSlots`, `(*Character).Items`; `EquipItem` wears an owned item in a slot it names; the two loads accept or drop items — [rpg-project#1162](https://github.com/KirkDiggler/rpg-project/issues/1162).

## Decided

- `Clone` methods are exported on `Description` and `Data`, because "everything returned or stored is copied" needs one place that copies, and `character` calls it — [issue Design 1](https://github.com/KirkDiggler/rpg-project/issues/1162).
- `WornSlots` is exported from `character`, so a caller authoring an item can read which slots are legal — [issue Design 5](https://github.com/KirkDiggler/rpg-project/issues/1162).
- A lenient load of a repeated id keeps the first item and drops the later one; a slot naming the id still names the kept one — [issue Design 2](https://github.com/KirkDiggler/rpg-project/issues/1162).
- A lenient load does not clear a slot whose id is a catalogue id the inventory holds: a dropped item with that id never displaced the inventory row — [issue Design 2](https://github.com/KirkDiggler/rpg-project/issues/1162).
- The caller's `EquipmentSlots` map is copied before a lenient load clears a slot, so the caller's data is not edited — [issue Design 2](https://github.com/KirkDiggler/rpg-project/issues/1162).

## Found

- F7 printed 36 lines; no reader walks every slot and looks an id up in the catalogue, so no gap — [comment](https://github.com/KirkDiggler/rpg-project/issues/1162#issuecomment-7148949717).
- Test 1's bytes were captured from a fighter with a longsword in hand loaded with `Load`, `UpdatedAt` set to `CreatedAt` because `ToData` stamps the clock, before `data.go` changed — [issue Test 1](https://github.com/KirkDiggler/rpg-project/issues/1162).
- Proof 1: making `slotsAfterWearing` ignore the item's `Slots` fails `TestACloakIsWornInItsSlotAndTakenOffKeepingItsCount`; proof 2: dropping `Left` from what `ToData` writes fails `TestTheRecordSurvivesSaveAndLoadAndTheItemStaysWorn` — [issue proofs](https://github.com/KirkDiggler/rpg-project/issues/1162).
- `go test ./character/... ./features/...` passed before (at `28b42774`, 0.31s for `character`) and `./magicitems/... ./character/... ./features/...` passes after; the whole-module suite passes at the final head; `go build`, `go vet`, `golangci-lint` (0 issues), and `go test -race ./magicitems/...` (7.3s) are clean — [issue commands](https://github.com/KirkDiggler/rpg-project/issues/1162).

## Mistaken

- My first lenient-load test assumed every dropped id clears its slot; it cleared the inventory's longsword slot for a catalogue-id item and the test said so before the code shipped. The test now has one case per outcome — [issue Test 8](https://github.com/KirkDiggler/rpg-project/issues/1162).
