# Run — 2026-11-28 — second ability 5c-1b a worn item's ability may charge no slot

Slice [rpg-project#1263](https://github.com/KirkDiggler/rpg-project/issues/1263) of story [rpg-project#1131](https://github.com/KirkDiggler/rpg-project/issues/1131). Base: `rpg-toolkit` `main` at [`17d3acbd`](https://github.com/KirkDiggler/rpg-toolkit/commit/17d3acbd).

One line per entry. Every line ends with a link.

## Shipped

- `grantedEntry` makes a worn item's ability that charges no slot a free-action row (`ActionFree`, `EconomySlotFree`), usable while the item's count allows; one slot is as before; more than one is refused with "must cost at most one slot" — [rpg-project#1263](https://github.com/KirkDiggler/rpg-project/issues/1263).

## Decided

- Out of combat a free row is not listed, because `AvailableAbilities` returns an empty list for every row when not in combat, so a free row is judged as a slot row is — [rpg-project#1263](https://github.com/KirkDiggler/rpg-project/issues/1263).
- Test 6 (a two-slot description) is left out: no description with two slots can be built from this package without a production seam, which the issue forbids — [rpg-project#1263](https://github.com/KirkDiggler/rpg-project/issues/1263).

## Found

- F3: "exactly one slot" appears once in the whole toolkit, in `worn_items.go` itself; no test in any module pins it, so no existing test is edited — [rpg-project#1263 first comment](https://github.com/KirkDiggler/rpg-project/issues/1263#issuecomment-7231445290).
- Design 2: no second place in `character` assumes an item's row costs a slot; `status_view.go` and `equipment_display.go` read the description's name and the item's pools only, and show "Action Surge · Surge cloak" (status pool) and "Action Surge · 1 of 1 charges" (equipment stat line) — [rpg-project#1263 first comment](https://github.com/KirkDiggler/rpg-project/issues/1263#issuecomment-7231445290).
- Before the change, tests 1 to 4 failed on real assertions (ActionType "" not "free", CanUse false, reason "must cost exactly one slot"); tests 5 and 7 passed as expected — [rpg-project#1263](https://github.com/KirkDiggler/rpg-project/issues/1263).
- Suite before (build, vet, `go test ./...`) passed at `17d3acbd` — [rpg-project#1263 first comment](https://github.com/KirkDiggler/rpg-project/issues/1263#issuecomment-7231445290).

- Proof (a), the old refusal restored (any cost other than exactly one slot refused): the four tests for offered-free, needs-no-action, needs-the-count and paying all failed — [rpg-project#1263](https://github.com/KirkDiggler/rpg-project/issues/1263).
- Proof (b), a free row given the standard action's check: the offered-free test failed (`expected "free" actual "action"`) and the needs-neither-action test failed (`Should be true`) — [rpg-project#1263](https://github.com/KirkDiggler/rpg-project/issues/1263).
- Suite at the final head: build, vet, `go test ./... -count=1 -timeout 20m` and `golangci-lint run ./...` all pass — [rpg-project#1263](https://github.com/KirkDiggler/rpg-project/issues/1263).

## Mistaken

- none observed — [rpg-project#1263](https://github.com/KirkDiggler/rpg-project/issues/1263).
