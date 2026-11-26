# Run — 2026-11-27 — NPC turn 1d-1 the rules price a monster's Dash

Slice [rpg-project#1206](https://github.com/KirkDiggler/rpg-project/issues/1206) of story [rpg-project#1131](https://github.com/KirkDiggler/rpg-project/issues/1131). Base: `rpg-toolkit` `main` at [`76627708`](https://github.com/KirkDiggler/rpg-toolkit/commit/76627708).

## Shipped

- `monster.CostOfDash`: one action for every sheet, no grant, a fresh profile per call, nil refused by name; no caller yet and nothing in play changes — [rpg-project#1206](https://github.com/KirkDiggler/rpg-project/issues/1206).

## Decided

- The profile has no Grants: an NPC's movement is not on its sheet, the encounter adds a run's speed to its own count, and the day it is on the sheet this profile banks it — [ruling 2026-11-27](https://github.com/KirkDiggler/rpg-project/issues/1131#issuecomment-7200191665).
- The sheet is taken though not read, for the reason CostOfAction gives — [rpg-project#1206](https://github.com/KirkDiggler/rpg-project/issues/1206).

## Found

- F1 holds: `CostOfAction`, `CostOfDisengage` and `oneSlot` are at spend_profile.go lines 60, 101 and 121 of origin/main — [comment](https://github.com/KirkDiggler/rpg-project/issues/1206#issuecomment-7200212320).
- F2 holds: `git grep "func CostOfDash" origin/main -- rulebooks/dnd5e` printed nothing — [comment](https://github.com/KirkDiggler/rpg-project/issues/1206#issuecomment-7200212320).
- F3 holds: a character's Dash costs one action and adds its speed to its movement remaining via `AddMovement` — [dash.go](https://github.com/KirkDiggler/rpg-toolkit/blob/76627708/rulebooks/dnd5e/combatabilities/dash.go#L67-L77).
- Proof A (Dash priced as one bonus action) failed TestADashIsExactlyOneActionAndGrantsNothing, TestEachDashPriceIsItsOwnProfile and TestTheGateTakesADashAndLeavesTheBonusActionAndReaction — [rpg-project#1206](https://github.com/KirkDiggler/rpg-project/issues/1206).
- Proof B (a movement grant added) failed TestADashIsExactlyOneActionAndGrantsNothing and TestEachDashPriceIsItsOwnProfile on the profile not equalling one action — [rpg-project#1206](https://github.com/KirkDiggler/rpg-project/issues/1206).
- `go test ./... -count=1` in rulebooks/dnd5e passed before the change; the result at the final head is in the PR body — [rpg-project#1206](https://github.com/KirkDiggler/rpg-project/issues/1206).

## Mistaken

- Restoring a mutant with `git checkout --` also dropped the uncommitted implementation, and the first proof B mutant had a syntax slip; the implementation was reapplied from a saved copy and proof B rerun — [rpg-project#1206](https://github.com/KirkDiggler/rpg-project/issues/1206).
