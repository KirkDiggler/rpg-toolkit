# Run — 2026-11-28 — second ability S5d-1 a creature uses Action Surge; the wearer is blocked

Slice [rpg-project#1261](https://github.com/KirkDiggler/rpg-project/issues/1261) of story [rpg-project#1131](https://github.com/KirkDiggler/rpg-project/issues/1131). Base: `rpg-toolkit` `main` at [`17d3acbd`](https://github.com/KirkDiggler/rpg-toolkit/commit/17d3acbd).

## Shipped

- The pins `rulebooks/dnd5e` v0.247.0 and `rulebooks/dnd5e/resolution` v0.86.0; nothing else in go.mod or go.sum moved — [slice](https://github.com/KirkDiggler/rpg-project/issues/1261).
- `testdata/lieutenant-surges-walk.yaml` and `action_surge_npc_test.go` (Tests 1 to 5 and 7); Test 6 (the wearer) is not written, see Found — [gap](https://github.com/KirkDiggler/rpg-project/issues/1261#issuecomment-7231382103).
- Production files changed in this module: none — [slice](https://github.com/KirkDiggler/rpg-project/issues/1261).

## Decided

- `selector_projection_test.go` lists `combat.SpendProfile.GrantSlots` as mechanical: the projection reuses `SpendProfile` whole, so the field already reaches the selector; the only edit to an existing test, no assertion changed — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1261#issuecomment-7231312349).
- Test 3's far placement is 8 cells (q 0), because at 7 cells he walks the whole way and surges on arrival — [gap comment](https://github.com/KirkDiggler/rpg-project/issues/1261#issuecomment-7231382103).

## Found

- F3 at the new pins: one failing test, `TestSelectorProjectionClassifiesEveryDefinitionField`: `Should be empty, but was [combat.SpendProfile.GrantSlots]` — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1261#issuecomment-7231312349).
- F4: `one of second wind` is in two run records only; `BankCapacity(` is nowhere; `NumField()` tests passed except the selector projection's — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1261#issuecomment-7231312349).
- F6: `afford.go:839-860` never reads `GrantSlots`; a worn item's grant is refused earlier, at `rulebooks/dnd5e/character/worn_items.go:165-169` (v0.247.0): "must cost exactly one slot", so the wearer's row is `Available: false` and `Activate` returns `declaration is stale` — [gap](https://github.com/KirkDiggler/rpg-project/issues/1261#issuecomment-7231382103).
- The turn he surges in (first die script, every die min(size, 10), site at 3 cells): `turn-ended answered moved moved answered activated activation-result answered struck heard answered struck heard answered stayed turn-ended tick`; one `activated` (Action Surge), one `capacity-granted` result with description `1 action`, two swings; his stored `Left["action-surge"]` 0 — [Test 1](https://github.com/KirkDiggler/rpg-project/issues/1261).
- His next turn: `turn-ended answered struck heard answered stayed turn-ended tick`: no surge, one swing — [Test 2](https://github.com/KirkDiggler/rpg-project/issues/1261).
- With the pick on the last candidate (faceDice top face): `moved moved answered struck heard answered stayed turn-ended tick`: he swings once, does not surge, and never does over three turns; he does not surge between swings — [Test 4](https://github.com/KirkDiggler/rpg-project/issues/1261).
- At 8 cells her out of a turn's walk: `turn-ended answered turn-ended tick`; at 7 cells he walks six steps, surges and swings twice — [Test 3](https://github.com/KirkDiggler/rpg-project/issues/1261).
- A player is sent one `ActivateAct`: By member `lieutenant`, Ability `Action Surge` (`dnd5e:features:action_surge`), no target, one result: kind `capacity-granted`, target `lieutenant`, description `1 action`, every number 0 — [Test 5](https://github.com/KirkDiggler/rpg-project/issues/1261).
- An uncounted stat block is refused by the rules through the assembly, at the check (`Templates`) and at the launch, with: `stat block "gate-lieutenant": lets[0]: \`use\` is "action surge", which gives its user something for its count alone; write \`uses\` or a \`cost\``; the launch error is not `ErrUnknownContent` and nothing is stored — [Test 7](https://github.com/KirkDiggler/rpg-project/issues/1261).
- A misspelt row `actoin surge` is refused offering `action surge, second wind` — [Test 7](https://github.com/KirkDiggler/rpg-project/issues/1261).
- At the old pins (v0.244.0, v0.85.0) Test 1 and the check test fail: the launch says `lets[0]: \`use\` is "action surge"; it is one of second wind` — [Tests 1 and 7](https://github.com/KirkDiggler/rpg-project/issues/1261).
- The suite passed at the old pins (62s) before the change — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1261#issuecomment-7231312349).

## Mistaken

- Test 3 and Test 4 were first written to the issue's expectation (he walks at 8 cells; the surge happens whichever way the die lands), and failed on the first run; they were rewritten to what the run showed — [gap](https://github.com/KirkDiggler/rpg-project/issues/1261#issuecomment-7231382103).
