# Run — 2026-11-28 — second ability S8e a creature and a wearer rage

Slice [rpg-project#1292](https://github.com/KirkDiggler/rpg-project/issues/1292) of story [rpg-project#1131](https://github.com/KirkDiggler/rpg-project/issues/1131). Base: `rpg-toolkit` `main` at [`1c2ae52d`](https://github.com/KirkDiggler/rpg-toolkit/commit/1c2ae52d).

## Shipped

- The pins `rulebooks/dnd5e` v0.253.0 and `rulebooks/dnd5e/resolution` v0.88.0; `encounter` stays v0.267.0 — [slice](https://github.com/KirkDiggler/rpg-project/issues/1292).
- `abilityFactsOf` in `sheets.go`: a creature's fact is payable only when `a.Payable && a.Refused == ""`; the one production change — [Design 2](https://github.com/KirkDiggler/rpg-project/issues/1292).
- `rage_npc_test.go` (the creature, Tests 1 to 5) and `rage_wearer_test.go` (the wearer, Tests 6 to 8); no new site, the raging Lieutenant is the second-wind site with two texts replaced — [slice](https://github.com/KirkDiggler/rpg-project/issues/1292).
- One sentence in `AGENTS.md` where it says what `SheetFacts.Abilities` carries — [Design 4](https://github.com/KirkDiggler/rpg-project/issues/1292).

## Decided

- No new site: `usesVariant` over `lieutenant-uses-second-wind-walk.yaml` says it (the `lets` entry and the first row replaced), so the raging Lieutenant cannot drift from the second-wind one — [slice](https://github.com/KirkDiggler/rpg-project/issues/1292).
- Test 4 reads "his fact is not payable" through `monster.AbilitiesOf` (`Payable` true, `Refused` `already raging`) and through what he does (no activated beat), because `abilityFactsOf` is unexported and the allowed files hold no internal test — [slice](https://github.com/KirkDiggler/rpg-project/issues/1292).
- Test 8 asserts `ErrStaleDeclaration` for the wearer's second use, not `ErrCannotActivate`, because the offer is unavailable and the verb refuses it first — [gap](https://github.com/KirkDiggler/rpg-project/issues/1292#issuecomment-7236390693).

## Found

- F1: `go.mod` at base printed `v0.251.0`, `v0.267.0`, `v0.87.1`; both tags `rulebooks/dnd5e/v0.253.0` and `rulebooks/dnd5e/resolution/v0.88.0` exist — [Rests on](https://github.com/KirkDiggler/rpg-project/issues/1292#issuecomment-7236351182).
- F2: `sheets.go:455` `func abilityFactsOf` and `sheets.go:465` `Payable: a.Payable`; `monster.AbilitiesOf` is read outside tests at `activate_npc.go:60` (the word and the description; stands) and `sheets.go:456` (the one change) — [Rests on](https://github.com/KirkDiggler/rpg-project/issues/1292#issuecomment-7236351182).
- F6: `action_surge_npc_test.go:264` pins `one of action surge, second wind` in the refusal of a misspelt word (edited: `rage` in its sorted place); `knowledge/runs/2026-11-28-a-creature-and-a-wearer-use-action-surge.md:29` records the old sentence in the past and is left alone — [Rests on](https://github.com/KirkDiggler/rpg-project/issues/1292#issuecomment-7236351182).
- At the pins, before any other change, one test failed: `TestTheContentCheckAndTheLaunchJudgeActionSurge/a_misspelt_word`, `action_surge_npc_test.go:264`: `it is one of action surge, rage, second wind` does not contain `one of action surge, second wind` — [Rests on](https://github.com/KirkDiggler/rpg-project/issues/1292#issuecomment-7236351182).
- Test 4 before the change fails with the door's refusal surfacing as a failed turn: `endturn: end turn "alice": participation drive: drive monster turns "lieutenant": execute: activate: activate npc: ability cannot be activated: resolution: ability refused to activate: already raging` — [Test 4](https://github.com/KirkDiggler/rpg-project/issues/1292).
- Test 5 can see the bonus: a creature's `struck` beat carries `damage_components`; the raging Lieutenant's longsword hit lists weapon d8 (8), ability strength (0) and one `condition` component sourced to `dnd5e:conditions:raging` and `lieutenant` with modifier 2, `amount` 10 — [Test 5](https://github.com/KirkDiggler/rpg-project/issues/1292).
- A raging wearer's row: `Available` false, `Why.Text` `already raging`; `Manager.Activate` on it returns `activate: declaration is stale` — [gap](https://github.com/KirkDiggler/rpg-project/issues/1292#issuecomment-7236390693).
- His first turn: `activated` (Rage), one `activation-result` (`condition-applied`, target `lieutenant`, `Raging`), then he walks and swings; his sheet has `Left["rage"]` 1, no bonus action, the raging status — [Test 2](https://github.com/KirkDiggler/rpg-project/issues/1292).
- The proof (`abilityFactsOf` copying `a.Payable` alone again, restored after) fails Test 4 with the same message as before the change: `... ability refused to activate: already raging`; the suite passes at the pins' first run except the one test above, and at the final head (62.7s), with `go build`, `go vet` and `golangci-lint run` clean — [Proof](https://github.com/KirkDiggler/rpg-project/issues/1292).

## Mistaken

- Test 8 was first written to the issue's `ErrCannotActivate` and failed with `declaration is stale`; it was rewritten to what the code does and the difference reported as a gap — [gap](https://github.com/KirkDiggler/rpg-project/issues/1292#issuecomment-7236390693).
