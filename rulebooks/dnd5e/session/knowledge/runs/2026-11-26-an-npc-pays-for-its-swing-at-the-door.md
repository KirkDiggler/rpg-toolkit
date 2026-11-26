# Run — 2026-11-26 — NPC turn 1c-2 an NPC's swing and its Disengage are paid from its sheet at the door

Slice [rpg-project#1198](https://github.com/KirkDiggler/rpg-project/issues/1198) of story [rpg-project#1131](https://github.com/KirkDiggler/rpg-project/issues/1131). Base: `rpg-toolkit` `main` at [`a936f9fa`](https://github.com/KirkDiggler/rpg-toolkit/commit/a936f9fa).

One line per entry. Every line ends with a link.

## Shipped

- `strikerSeam.Strike` always sends the price `monster.CostOfAction` reads, and `strikerSeam.Disengage` sends `monster.CostOfDisengage`'s, both paid from the monster's stored sheet at the door; pins `rulebooks/dnd5e` v0.239.0 and `resolution` v0.83.0 — [rpg-project#1198](https://github.com/KirkDiggler/rpg-project/issues/1198).
- Tests 1 to 5 in `npc_pays_test.go` (internal package) and the rewritten price test in `attack_internal_test.go` — [rpg-project#1198](https://github.com/KirkDiggler/rpg-project/issues/1198).

## Decided

- `npc_pays_test.go` is in the internal package, because Tests 1 to 4 need `openForWrite` and the unexported strike seam, and Test 5 plays Launch, Afford, Move and EndTurn from the same package on the internal fixtures — [rpg-project#1198](https://github.com/KirkDiggler/rpg-project/issues/1198).
- A second strike or Disengage refused at the door returns `ErrCannotAfford` after translation, asserted in the tests — [rpg-project#1198](https://github.com/KirkDiggler/rpg-project/issues/1198).
- Test 5 is two played fights, one where the goblin disengages and walks and one where it swings, so each seam's turn start is played — [rpg-project#1198](https://github.com/KirkDiggler/rpg-project/issues/1198).
- `TestASecondDisengageDoesNotStack` now says the second Disengage is refused; the later-turn replace claim is left to resolution's `TestASecondDrivenDisengageReplacesTheFirst`, because this fixture holds no clock — [rpg-project#1198](https://github.com/KirkDiggler/rpg-project/issues/1198).

## Found

- F1 printed `rulebooks/dnd5e v0.236.0`, `encounter v0.261.0`, `resolution v0.82.0`; F2 printed `CostOfAction` at `spend_profile.go:60` and `CostOfDisengage` at `:101`; F3 printed the branch at `striker.go:115` and the comment at `disengage.go:41` — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1198#issuecomment-7186397544).
- F4 printed one production caller (`encounter/clocks.go:1169`) and two test callers (`attack_internal_test.go:781`, `disengage_internal_test.go:65`) — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1198#issuecomment-7186397544).
- F5 at the new pins: build passes, vet fails on `castflee_test.go:79` as predicted, and one more test failed that the issue did not list — [the gap](https://github.com/KirkDiggler/rpg-project/issues/1198#issuecomment-7186440056).
- F6, order: the encounter announces a member's TurnStarted when the clock advances, before that member's intents are driven (`clocks.go:1006` then `:1169`); resolution publishes it (`boundary.go:133`) and a loaded monster hears it (`monster/load.go:237`) — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1198#issuecomment-7186397544).
- F6, saved: the announcer's cast holds the stored monster sheets, resolution returns the changed ones as `DirtyMonsters`, and `land` saves them; the two-round tests confirm it, since a swing and a Disengage both happen a second time — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1198#issuecomment-7186397544).
- The goblin's stored sheet after its turn start and before it acts is not observable from a test, because a verb saves its sheets only at its commit — [rpg-project#1198](https://github.com/KirkDiggler/rpg-project/issues/1198).
- Proof (a), no cost on the swing: the rewritten price test, Test 1, Test 2 and the swing arm of Test 5 fail — [rpg-project#1198](https://github.com/KirkDiggler/rpg-project/issues/1198).
- Proof (b), no cost on the Disengage: Test 3, Test 4, `TestASecondDisengageDoesNotStack` and the Disengage arm of Test 5 fail — [rpg-project#1198](https://github.com/KirkDiggler/rpg-project/issues/1198).
- Proof (c), a `Turn` with number 99 on the monster's cost: nothing fails, the resolver ignores it for a monster — [rpg-project#1198](https://github.com/KirkDiggler/rpg-project/issues/1198).
- Test 6: `TestAGoblinMayDisengageAsABonusAction` (`sheets_internal_test.go`) holds that a wolf has no Nimble Escape, and passes unedited — [rpg-project#1198](https://github.com/KirkDiggler/rpg-project/issues/1198).

## Mistaken

- The issue listed the direct `Strike` call in a test (F4) without reading what it asserts; it pinned the refusal this story retires, and Amendment 1 made it an allowed edit — [Amendment 1](https://github.com/KirkDiggler/rpg-project/issues/1198#issuecomment-7186449168).
- A proof's `git checkout --` on `striker.go` while the change was uncommitted discarded it; it was restored from a copy and the work committed before the proofs — [rpg-project#1198](https://github.com/KirkDiggler/rpg-project/issues/1198).
