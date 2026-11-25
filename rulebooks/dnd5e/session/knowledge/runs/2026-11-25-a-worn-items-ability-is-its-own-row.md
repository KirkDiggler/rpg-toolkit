# Run — 2026-11-25 — item story S: a worn item's ability is its own row

processed: 2026-11-25T190427Z

Slice [rpg-project#1164](https://github.com/KirkDiggler/rpg-project/issues/1164) of story [rpg-project#850](https://github.com/KirkDiggler/rpg-project/issues/850). Base: `rpg-toolkit` `main` at [`bc93f94f`](https://github.com/KirkDiggler/rpg-toolkit/commit/bc93f94f3a9b1bc05410f1bfc24bb61e339e1723).

## Shipped

- `AbilityRef` gains an optional `Giver` (`AbilityGiver{ItemID, ItemName, Grant}`), so an Activate row says which worn item grants it — [rpg-project#1164](https://github.com/KirkDiggler/rpg-project/issues/1164).
- Two givers of one ability are two rows with two ids, named "Second Wind · Lieutenant's cloak", ordered by ref, the member's own first, then item id and grant — [rpg-project#1164](https://github.com/KirkDiggler/rpg-project/issues/1164).
- `Activate` asks the sheet for `GrantedAbility` when the selected row has a giver, and runs it through the described path; the payer's sheet pays the item's use — [rpg-project#1164](https://github.com/KirkDiggler/rpg-project/issues/1164).
- Pins `rulebooks/dnd5e` v0.233.0 and `rulebooks/dnd5e/resolution` v0.82.0: the diff is those two lines of `go.mod` plus `go.sum` — [rpg-project#1164 first comment](https://github.com/KirkDiggler/rpg-project/issues/1164#issuecomment-7149395849).

## Decided

- The giver's identity rides in the existing `ability` string of `selectorIDFor` as `ref|item:<len>:<id>|grant:<len>:<grant>`, built by one function, `activationIdentity`; `declaration_id.go` is untouched because the material is already a string — [rpg-project#1164](https://github.com/KirkDiggler/rpg-project/issues/1164).
- The member's own row passes the bare ref, so its id is byte for byte what it was — [rpg-project#1164](https://github.com/KirkDiggler/rpg-project/issues/1164).
- Both sorts (`activations.go`, `offers.go`) share one comparator, `abilityRowLess` — [rpg-project#1164](https://github.com/KirkDiggler/rpg-project/issues/1164).

## Found

- F1 to F8 all hold at `bc93f94f`; F5 printed `act_bodies.go:219`, `act_family.go:636`, `:750`, `activations.go:86`, `afford.go:342`, `types.go:1790-1939`, and no test pins the field list of the session's `AbilityRef` — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1164#issuecomment-7149395849).
- The suite passed on the pins alone with no other edit (session 60.3s) — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1164#issuecomment-7149395849).
- Test 1's id was captured by a throwaway test run on the unchanged `activations.go`: Afford for alice on turn 1 of `testdata/second-wind-walk.yaml`, session "sess", printed id `v2.B-E_FF4_AoxcpUkZHlAQQWFnPoVOrjnLkQXLC_slPeA`, name "Second Wind"; the test pins both — [rpg-project#1164](https://github.com/KirkDiggler/rpg-project/issues/1164).
- A launch starts the fighter at full health, so the tests hurt her after the launch by saving the sheet — [rpg-project#1164](https://github.com/KirkDiggler/rpg-project/issues/1164).
- Test 8: in a fight, `Equip` of the cloak into the cloak slot is refused as `ErrArmorInFight`, text `equip: body armour cannot change in a fight: resolution: equip "alice": equipment change refused in a fight: body armour cannot change in a fight`; free roam takes it and the row appears — [rpg-project#1164](https://github.com/KirkDiggler/rpg-project/issues/1164).
- Existing tests edited: none — [rpg-project#1164](https://github.com/KirkDiggler/rpg-project/issues/1164).
- Proof 1: building every row's identity from the ref alone fails Test 2 (`TestAWornCloakIsASecondWindRowBesideTheFightersOwn`), Test 3/4, Test 5 and Test 9; proof 2: making `Activate` ask `AbilityDefinition` for every row fails Test 3/4, Test 7 and Test 9; each restored after — [rpg-project#1164](https://github.com/KirkDiggler/rpg-project/issues/1164).
- Final head: `go build`, `go vet`, `go test ./... -count=1` (session 60.1s) and `golangci-lint run ./...` pass; `go test -race -run 'WornItem|SecondWindWalk|Activat'` passes in 49.8s wall — [rpg-project#1164](https://github.com/KirkDiggler/rpg-project/issues/1164).

## Mistaken

- The first draft of the tests started the fighter hurt in the saved data and read a full-health sheet after the launch; the mismatch showed as two failures and the fix is the post-launch save in `newWornFight` — [rpg-project#1164](https://github.com/KirkDiggler/rpg-project/issues/1164).
