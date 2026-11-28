# Run — 2026-11-28 — one way to pay for an ability S6a the fighter's Action Surge is counted in the shared pool and used from its description

Slice [rpg-project#1269](https://github.com/KirkDiggler/rpg-project/issues/1269) of story [rpg-project#1131](https://github.com/KirkDiggler/rpg-project/issues/1131). Base: `rpg-toolkit` `main` at [`c6b6bdcc`](https://github.com/KirkDiggler/rpg-toolkit/commit/c6b6bdcc2f9d41cbdc4d3d8a6cf5f780938f2e55).

## Shipped

- A fighter whose class level holds the Action Surge grant gets the shared `action_surge` pool (short rest, sized from the grant's `{"uses": n}`); `secondWindGrantUses` became `fighterGrantUses(classLevel, feature)` and serves both — [https://github.com/KirkDiggler/rpg-project/issues/1269](https://github.com/KirkDiggler/rpg-project/issues/1269).
- `features.ActionSurge` keeps no counter: it reports the owner's pool, `CanActivate` asks the pool, `Definition` is its price only (one of the pool, one more action), `Activate` refuses with `ErrUsedFromItsDescription`, `Apply` and `Remove` are gone — [https://github.com/KirkDiggler/rpg-project/issues/1269](https://github.com/KirkDiggler/rpg-project/issues/1269).
- `featureResourceInfo` reads the pool so her row shows 1 of 1; a fighter's owner rows are Second Wind, Action Surge and hit dice — [https://github.com/KirkDiggler/rpg-project/issues/1269](https://github.com/KirkDiggler/rpg-project/issues/1269).

## Decided

- Test 10 (an older save) is dropped by [amendment 1](https://github.com/KirkDiggler/rpg-project/issues/1269): no test builds data in the old shape.

- The giver (the Fighter) is passed to `DescribeFromRef` as the issue says; the built description carries no giver (Action Surge has no profile to name it on), so Test 4 asserts the price, the name and the prose and holds the giver only as "the build succeeded" — [design 2](https://github.com/KirkDiggler/rpg-project/issues/1269).
- `TestEveryFeatureWithAPoolIsInBothCatalogsItBelongsIn` moved the Action Surge row to owner-owned (case a: it reads where the count lives) — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1269#issuecomment-7232311414).
- The two attach tests moved their subject to Warding Flare on a Light cleric's sheet (the one feature left that subscribes to the bus; Deflect Missiles only keeps the bus), with Wrath of the Storm after it for the lenient test; both clerics' pools are on the synthetic sheet because a rest reads every feature's status — [case e](https://github.com/KirkDiggler/rpg-project/issues/1269).
- `ClassResourceActionSurge` and `GrantExtraAction` are not deleted — [design 5](https://github.com/KirkDiggler/rpg-project/issues/1269).

## Found

- Rests on: F1 to F3 and F8 hold at `c6b6bdcc`; the suite, `go vet` and `go build` passed before any change — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1269#issuecomment-7232311414).
- F4 (non-test hits, in full) and F5 (tests, with cases) are in the first comment; one comment is stale in `combat` (`action_economy_ledger.go:173` says the old feature still calls `GrantExtraAction`), left alone — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1269#issuecomment-7232311414).
- F7: after the change `GrantExtraAction` has no non-test caller (its tests are `combat/action_economy_test.go` and `combatabilities/attack_test.go`); `ClassResourceActionSurge` is named only where it is defined, `shared/types.go:143-144`, in every module — [issue](https://github.com/KirkDiggler/rpg-project/issues/1269).
- F6: the features that still subscribe to the bus in `features` are `DeflectMissiles` (stores the bus only), `WardingFlare` and `WrathOfTheStorm`; no fighter feature is among them — [issue](https://github.com/KirkDiggler/rpg-project/issues/1269).
- The level-up's `Gained.Resources` for a level-1 fighter taking level 2 was `[hit_dice 1→2]` and is `[action_surge 0→1, hit_dice 1→2]`; no test in any module pinned the old list — [Test 2](https://github.com/KirkDiggler/rpg-project/issues/1269).
- Tests 1 to 9 failed before the change (no pool, no refusal, row unusable with "action economy is required for action surge", the save refused the key) — [issue](https://github.com/KirkDiggler/rpg-project/issues/1269).
- Proofs (each made on the working tree after the commit, restored with `git checkout --`, never committed): (a) pool dropped from `buildClassResources`: the pool, level-up, row, gate, rest and save tests fail, plus three status-view tests; (b) Action Surge dropped from the fighter's owner rows: Test 9 fails with "resource action_surge is not in the fighter status-view owner catalog", with the catalog table and three status-view tests; (c) `Activate` returns nil: Test 6 and the features suite's refusal test fail — [issue](https://github.com/KirkDiggler/rpg-project/issues/1269).
- Existing tests: retired `TestPrivateResourceLoadsRejectInvalidBounds`, `TestStrictLoadRejectsFeaturePrivateResourceBoundsBeforeStatusView`, `TestLenientLoadDropsMalformedFeaturePrivateResource` (case d); the old `TestApplyRemove` and private-rest tests in `features/action_surge_test.go` (case d); edited `TestActionSurgeArrivesUsableAndIsSpendable`, `TestASpentActionSurgeReloadsSpent`, `TestALoadedSheetWithNoBusCanStillAdvance`, `TestActionSurgeStatusReportsOwnerPoolWithoutJSON` (cases a, b), `TestAttachRollsBackAFailedFeature`, `TestLenientAttachDropsFailedFeatureAndContinues`, `TestAttachScopesAttachableFeatureToItsRef` (case e), `TestEveryFeatureWithAPoolIsInBothCatalogsItBelongsIn` (case a) — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1269#issuecomment-7232311414).
- Tests in other modules that need a change at their pin: `resolution/long_rest_test.go` near `:130` (reads `ActionSurgeData.Uses`/`MaxUses`) and `:301` (marshals them) — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1269#issuecomment-7232311414).
- Suite at the final head: go build and go vet clean, go test ./... -count=1 -timeout 20m 41 packages ok and none failing, golangci-lint 0 issues; at the base all packages were ok too — [issue](https://github.com/KirkDiggler/rpg-project/issues/1269).

## Mistaken

- none observed beyond a first draft of Test 4 that asserted a giver the description does not carry; the run record says so in Decided — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1269#issuecomment-7232311414).
