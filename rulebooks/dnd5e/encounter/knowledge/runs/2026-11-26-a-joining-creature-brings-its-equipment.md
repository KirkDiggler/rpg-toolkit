# Run — 2026-11-26 — item story S4 a joining creature brings its equipment

Slice [rpg-project#1171](https://github.com/KirkDiggler/rpg-project/issues/1171) of story [rpg-project#850](https://github.com/KirkDiggler/rpg-project/issues/850). Base: `rpg-toolkit` `main` at [`f8d06010`](https://github.com/KirkDiggler/rpg-toolkit/commit/f8d06010).

## Shipped

- `JoinInput.Equipment`: `Join` refuses bad equipment as construction does, keeps a deep copy on the member's record, and a reserved join keeps it until it arrives; looting a joined creature gives its items — [slice](https://github.com/KirkDiggler/rpg-project/issues/1171).

## Decided

- The refusal is `join: <what>: ErrBadMember`, the shape `Join` already gives a bad member fact, asked in `validateJoin` before the first mutation — [Design 2](https://github.com/KirkDiggler/rpg-project/issues/1171).

## Found

- F1 to F5 hold at `f8d06010`; F5 prints one `encounter.JoinInput{` literal outside tests (`cmd/freeroam-workbench/main.go:520`) and nothing copies a `JoinInput` field by field — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1171#issuecomment-7173103033).
- F3: save, load, load-time validation and arrival (`arriveMember` registers the whole record) already carry equipment; only `field.go`, `validateJoin` and `admitMember` needed a line, `reserve.go` none — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1171#issuecomment-7173103033).
- Proofs: dropping the equipment in `admitMember` fails TestAJoinedCreatureKeepsItsEquipmentThroughASave, TestLootingAJoinedCreatureGivesItsEquipment, TestAReservedJoinKeepsItsEquipmentUntilItArrives and TestJoinKeepsACopyOfTheEquipment; clearing it when a reserve arrives fails only TestAReservedJoinKeepsItsEquipmentUntilItArrives — [slice](https://github.com/KirkDiggler/rpg-project/issues/1171).
- Suite: `go test ./... -count=1` passes on `origin/main` before the change; the tests alone failed on the missing behaviour before the code; `go test -race . -run 'Join|Loot|Equipment|Reserve'` passes in 50s; `golangci-lint run ./...` reports 0 issues; no existing test edited, no picture or hash moved — [slice](https://github.com/KirkDiggler/rpg-project/issues/1171).

## Mistaken

- The first baseline suite run was repeated by accident to read its tail; no effect on the result — [slice](https://github.com/KirkDiggler/rpg-project/issues/1171).
