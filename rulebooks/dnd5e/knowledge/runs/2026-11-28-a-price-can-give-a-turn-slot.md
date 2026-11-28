# Run — 2026-11-28 — second ability 5b-1 a price can give a turn slot

Slice [rpg-project#1257](https://github.com/KirkDiggler/rpg-project/issues/1257) of story [rpg-project#1131](https://github.com/KirkDiggler/rpg-project/issues/1131). Base: `rpg-toolkit` `main` at [`9481d75e`](https://github.com/KirkDiggler/rpg-toolkit/commit/9481d75e7c2eeb465ae838b48590cfccfd28da26).

One line per entry. Every line ends with a link.

## Shipped

- `SpendProfile.GrantSlots`, judged by `Validate` and counted by `movesNothing`, landed by `Pay` after every charge, answered by all four ledger holders, carried by `CloneSpendProfile` and the character's price merge, worded by `combat.GrantedWords` — [rpg-project#1257](https://github.com/KirkDiggler/rpg-project/issues/1257).

## Decided

- `GrantExtraAction` and `GrantExtraBonusAction` are left as they were and do not call the new method, because the old Action Surge feature is untouched in this slice (the issue's "Leave alone") — [rpg-project#1257](https://github.com/KirkDiggler/rpg-project/issues/1257).
- The character's price merge sums `GrantSlots` (the issue: "a merge that drops it is a defect: carry it") — [Design 4](https://github.com/KirkDiggler/rpg-project/issues/1257).
- `GrantedWords` lives in the new file `combat/granted_words.go` and says nothing about `Grants` (capacity), whose words stay with `describeGrantedCapacity` — [Design 5](https://github.com/KirkDiggler/rpg-project/issues/1257).

## Found

- F4 printed nine `BankCapacity` holders, all in the root dnd5e module and none in `encounter`, `resolution` or `session` — [comment](https://github.com/KirkDiggler/rpg-project/issues/1257#issuecomment-7230931578).
- F7: no test pins a price's whole JSON with the new field's place; the one `NumField()` test walks a table, not `SpendProfile`; nine compile-time ledger claims, all in this module — [comment](https://github.com/KirkDiggler/rpg-project/issues/1257#issuecomment-7230931578).
- `mockMonkCharacter` makes no ledger claim and compiles without the method, so it is not edited — [comment](https://github.com/KirkDiggler/rpg-project/issues/1257#issuecomment-7230931578).
- The character writes a slot straight onto its own economy (`actionEconomy.ActionsRemaining` and kin) and calls `economyChanged()`; the monster goes through `writable()` onto its `ActionEconomy`; the sheet ledger is read-only — [character/ledger.go](https://github.com/KirkDiggler/rpg-project/issues/1257).
- `resolution.chargesSomething` reads Slots, Capacity and Pools only, so a price that only gives a slot is "no payment" there; `resolution.samePrice` compares prices as JSON, so it sees the new field; `session` clones prices through `CloneSpendProfile` and reads `Slots`/`Capacity`/`Pools` in `afford.go` — none edited, listed for the lead — [comment](https://github.com/KirkDiggler/rpg-project/issues/1257#issuecomment-7230931578).
- The one encoder this module uses for a price is `encoding/json`; the field is tagged `omitempty` and a price without it marshals to exactly `{"slots":{"bonus_action":1}}` — [grant_slots_test.go](https://github.com/KirkDiggler/rpg-project/issues/1257).
- Tests seen failing on real assertions before the behaviour: gate order (missing grant), the gift paying for itself, validate/CanPay accepting bad keys, economy/character/monster slot counts staying 1, clone and merge returning nil, words returning empty — [rpg-project#1257](https://github.com/KirkDiggler/rpg-project/issues/1257).
- PROOFS_AND_SUITE_PLACEHOLDER

## Mistaken

- none observed — [rpg-project#1257](https://github.com/KirkDiggler/rpg-project/issues/1257).
