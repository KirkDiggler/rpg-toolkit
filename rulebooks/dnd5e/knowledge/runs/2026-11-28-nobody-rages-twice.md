# Run — 2026-11-28 — nobody rages twice S8b held-status refusal

Slice [rpg-project#1289](https://github.com/KirkDiggler/rpg-project/issues/1289) of story [rpg-project#1131](https://github.com/KirkDiggler/rpg-project/issues/1131). Base: `rpg-toolkit` `main` at [`af9798ac`](https://github.com/KirkDiggler/rpg-toolkit/commit/af9798ac32bb0f9cbb42dc0fdd35c8b0d59ed2b3).

## Shipped

- `Definition.StatusOnUser`, `features.RefuseHeldStatus` and `RefuseHeldStatusStored`, asked by a worn item's use and row, a creature's use, and `monster.AbilitiesOf` (new `Ability.Refused`) — [slice issue](https://github.com/KirkDiggler/rpg-project/issues/1289).
- `Rage.refuseOwner` takes its reason from the same helper; the barbarian's words are unchanged — [slice issue](https://github.com/KirkDiggler/rpg-project/issues/1289).

## Decided

- The reason is `already ` + the lower-cased display name, `already holding <ref>` with no display — [Design 2 of the brief](https://github.com/KirkDiggler/rpg-project/issues/1289).
- The held status wins over the slot's and the count's reasons on a worn item's row — [Design 3 of the brief](https://github.com/KirkDiggler/rpg-project/issues/1289).
- `Payable` is unchanged by `Refused`; "can it be used right now" is `Payable` and `Refused == ""` — [Design 3 of the brief](https://github.com/KirkDiggler/rpg-project/issues/1289).

## Found

- F2: `DisplayFor(*refs.Conditions.Raging())` is `Display{Name: "Raging"}`, so the reason is `already raging` — [Rests on](https://github.com/KirkDiggler/rpg-project/issues/1289#issuecomment-7236162087).
- F5: the door's stand-in is `useRageFromItsDescription` in `character/rage_helper_test.go`; a creature rages by the status published on its bus, and its stored sheet is `ToData()` — [Rests on](https://github.com/KirkDiggler/rpg-project/issues/1289#issuecomment-7236162087).
- Design 4: `session/activate_npc.go:60` (matches `Word`, takes `Definition`) and `session/sheets.go:456` (copies `Word`, `Name`, `Payable`) call `AbilitiesOf`; none names `monster.Ability`; `encounter` has no hit; none edited — [Rests on](https://github.com/KirkDiggler/rpg-project/issues/1289#issuecomment-7236162087).
- Raging with no bonus action left: the barbarian's own row reads `no bonus action remaining` (the slot comes first there) and the item's row reads `already raging`; the two differ — [slice issue](https://github.com/KirkDiggler/rpg-project/issues/1289).
- `AbilitiesOf` puts the text of any error from `RefuseHeldStatusStored`, including an unreadable stored condition, in `Refused`, so a sheet it cannot read is never listed as free of refusal — [slice issue](https://github.com/KirkDiggler/rpg-project/issues/1289).
- Proof (a) `StatusOnUser` always false: the three features tests, `TestAWearerRagingFromAnItemIsNotOfferedRageAgain`, two barbarian-wearer cases and both monster tests fail — [slice issue](https://github.com/KirkDiggler/rpg-project/issues/1289).
- Proof (b) `grantedEntry` does not ask: the wearer's row fails (`Should be false`; reason `""` not `already raging`) and the barbarian rows fail; the use assertions still hold — [slice issue](https://github.com/KirkDiggler/rpg-project/issues/1289).
- Proof (c) `AbilitiesOf` does not ask: `TestAStoredRagingSheetListsRageRefusedAndStillPayable` fails (`Refused` `""`, wanted `already raging`) — [slice issue](https://github.com/KirkDiggler/rpg-project/issues/1289).
- Suite: `go build`, `go vet` and `go test ./... -count=1` passed before any change and at the final head; `golangci-lint run ./...` reports 0 issues — [slice issue](https://github.com/KirkDiggler/rpg-project/issues/1289).

## Mistaken

- none observed — [slice issue](https://github.com/KirkDiggler/rpg-project/issues/1289).
