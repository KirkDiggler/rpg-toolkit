# Run — 2026-11-28 — one way to pay for an ability S7c Rage is described and used from its description

Slice [rpg-project#1279](https://github.com/KirkDiggler/rpg-project/issues/1279) of story [rpg-project#1131](https://github.com/KirkDiggler/rpg-project/issues/1131). Base: `rpg-toolkit` `main` at [`da51cfac`](https://github.com/KirkDiggler/rpg-toolkit/commit/da51cfac).

## Shipped

- `(*Rage).Definition`: a bonus action and one `rage_charges` (no pool at barbarian 20), the raging status on its user with `{"damage_bonus":{"barbarian_level":true}}`; it validates; `var _ Described = (*Rage)(nil)` — [https://github.com/KirkDiggler/rpg-project/issues/1279](https://github.com/KirkDiggler/rpg-project/issues/1279).
- `(*Rage).Activate` refuses with `ErrUsedFromItsDescription`; the build, spend and publish are deleted; no production change in `character` — [https://github.com/KirkDiggler/rpg-project/issues/1279](https://github.com/KirkDiggler/rpg-project/issues/1279).

## Decided

- The two checks of who she is and what she holds (already raging, barbarian level) are one private function, `refuseOwner`, that `Definition` and `CanActivate` both call; neither it nor `Definition` asks for a charge — [design 1](https://github.com/KirkDiggler/rpg-project/issues/1279).
- Rage is not added to `named_abilities.go` — [design 1](https://github.com/KirkDiggler/rpg-project/issues/1279).
- An older save is neither handled nor tested, by the operator's ruling of 2026-11-28 — [ruling](https://github.com/KirkDiggler/rpg-project/issues/1131#issuecomment-7232312437).
- Case (c) moved to Flurry of Blows on a monk sheet (`TestAFeatureWithNoDescriptionIsNotAnError`, `TestAFeatureWithNoDescriptionIsUntouched`; assertions unchanged: no description, ki spent, bonus action taken) — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1279#issuecomment-7234464890).
- A sheet with no turn pays the helper's bonus action from a fresh turn's one (`freshTurnLedger`), as the integration twin of Second Wind's helper does — [design 4](https://github.com/KirkDiggler/rpg-project/issues/1279).

## Found

- Rests on: F1, F2, F3 (no other caller), F4, F6, F8 hold at `da51cfac`; build, vet and the suite passed before any change — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1279#issuecomment-7234464890).
- F3, F4, F6 and F7 as printed are in the first comment — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1279#issuecomment-7234464890).
- Edited by case: (a) `TestCharacterReceivesRageCondition`, `TestCharacterRemovesExpiredCondition`, `TestCharacterIgnoresOtherCharacterRemovals`, `TestNativeFirstTurnIsSavedThroughTheSharedCreator`, `TestRageReadsUnavailableWhileRaging`, `TestCanActivate`, `Example`, `TestEncounter_MultiTurnCombat`, `TestRage_EndsAfter10Turns`; (b) `TestActivatingRageWhileRagingIsRefused`, `TestRageRefusedWhileAlreadyRaging`, `TestRageRefusesAnOwnerWithNoBarbarianLevels`, `TestUnlimitedRagesAtLevel20` (rewritten onto the helper; Tests 2 and 8 hold the cost); (b) retired `TestActivatePublishesCondition` (Tests 1 and 4 hold the published status and its bindings); (c) the two above — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1279#issuecomment-7234464890).
- Test 5: the status's behaviour is still asserted by `TestEncounter_MultiTurnCombat` (+2 on the damage fold), `TestRage_EndsAfter10Turns` (ends after ten rounds) and `conditions/raging_test.go` (resistance), the first two now through the helper — [issue](https://github.com/KirkDiggler/rpg-project/issues/1279).
- Proofs: see the PR body; each made on the working tree after the commit and restored with `git checkout --` — [issue](https://github.com/KirkDiggler/rpg-project/issues/1279).
- Suite at the base: build, vet and `go test ./... -count=1 -timeout 20m` all ok; at the final head the same, golangci-lint 0 issues — [issue](https://github.com/KirkDiggler/rpg-project/issues/1279).

## Mistaken

- The first character test for "already raging" read the row's reason after the bonus action was spent and got "no bonus action remaining"; the tests now give two bonus actions so the refusal is Rage's alone — [issue](https://github.com/KirkDiggler/rpg-project/issues/1279).
