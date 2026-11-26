# Run — 2026-11-26 — item story S the session acts on unique

Slice [rpg-project#1182](https://github.com/KirkDiggler/rpg-project/issues/1182) of story [rpg-project#850](https://github.com/KirkDiggler/rpg-project/issues/850). Base: `rpg-toolkit` `main` at [`16f05310`](https://github.com/KirkDiggler/rpg-toolkit/commit/16f05310).

## Shipped

- A looted item gets the site's id only when its author says `unique: true`; any other gets `takenItemID` (session, body, item), so each is its own item — [https://github.com/KirkDiggler/rpg-project/issues/1182](https://github.com/KirkDiggler/rpg-project/issues/1182).
- `testdata/cloak-and-ring-walk.yaml`: a unique cloak and a ring that is not, for the operator to play (two runs: one cloak, two rings) — [https://github.com/KirkDiggler/rpg-project/issues/1182](https://github.com/KirkDiggler/rpg-project/issues/1182).
- The cloak in `lieutenants-cloak-walk.yaml` and `two-lieutenants-walk.yaml` says `unique: true` — [https://github.com/KirkDiggler/rpg-project/issues/1182](https://github.com/KirkDiggler/rpg-project/issues/1182).

## Decided

- `leaveFor` does not read `Unique`: a sheet saved under session v0.214.0 holds the cloak under the site's id with no `unique`, and leaving it keeps a later loot from being refused (Design 2, pinned by `TestASheetFromBeforeTheWordStillLeavesTheCloak`) — [https://github.com/KirkDiggler/rpg-project/issues/1182](https://github.com/KirkDiggler/rpg-project/issues/1182).
- `loot.go` changed on one call line only (`deliverTaken` takes the session id, `scope.session`); no signature of `deliverLoot` moved — [https://github.com/KirkDiggler/rpg-project/issues/1182](https://github.com/KirkDiggler/rpg-project/issues/1182).

## Found

- F0 held: go.mod at main had rulebook v0.234.0, encounter v0.260.0, resolver v0.82.0 — [comment](https://github.com/KirkDiggler/rpg-project/issues/1182#issuecomment-7183061653).
- F3: on the pins alone (`go mod tidy`) `go build ./...` and `go test ./... -count=1` pass (session 60.1s); no test pins the item read-back sentence or the item word list — [comment](https://github.com/KirkDiggler/rpg-project/issues/1182#issuecomment-7183061653).
- F4: after the two one-line site edits and no code change, one test failed: `TestTheLieutenantsCloakIsLootedWornAndUsed` pins `Unique:false` through `wornDescription`; every test in `loot_unique_test.go` passed — [comment](https://github.com/KirkDiggler/rpg-project/issues/1182#issuecomment-7183061653).
- go.mod diff: `rulebooks/dnd5e` v0.234.0 to v0.236.0, `rulebooks/dnd5e/encounter` v0.260.0 to v0.261.0, and their go.sum lines; nothing else moved — [https://github.com/KirkDiggler/rpg-project/issues/1182](https://github.com/KirkDiggler/rpg-project/issues/1182).
- Newest session tag before this PR is v0.214.0, so the title makes v0.215.0 — [comment](https://github.com/KirkDiggler/rpg-project/issues/1182#issuecomment-7183061653).
- Proof 1, every item gets the site's id (`if true`): `TestAUniqueCloakIsOnePerCharacterAndANotUniqueRingIsEveryTime` fails with `"[garrison-ring-49992cb5]" should have 2 item(s), but has 1` ("a second ring") — [https://github.com/KirkDiggler/rpg-project/issues/1182](https://github.com/KirkDiggler/rpg-project/issues/1182).
- Proof 2, every item gets the taken id (`if false`): four tests fail, among them `TestALooterOfTwoBodiesTakesOneCloak` with `"[lieutenants-cloak-a41acbdc lieutenants-cloak-66b629d5]" should have 1 item(s), but has 2` ("one cloak on the sheet") — [https://github.com/KirkDiggler/rpg-project/issues/1182](https://github.com/KirkDiggler/rpg-project/issues/1182).
- New tests, each under 0.1s: TestTakenItemID 0.00s, TestDeliverTakenPicksTheIDByTheWord 0.00s, TestAUniqueCloakIsOnePerCharacterAndANotUniqueRingIsEveryTime 0.05s, TestLootAnswersTheSameWhateverTheLooterOwnedOfCloakAndRing 0.05s, TestASheetFromBeforeTheWordStillLeavesTheCloak 0.03s — [https://github.com/KirkDiggler/rpg-project/issues/1182](https://github.com/KirkDiggler/rpg-project/issues/1182).
- At the final code head the build, vet, golangci-lint (0 issues) and `go test ./... -count=1 -timeout 20m` pass; the NUL join cannot tell `a\x00b|c` from `a|b\x00c`, as before; ids cannot hold a NUL — [https://github.com/KirkDiggler/rpg-project/issues/1182](https://github.com/KirkDiggler/rpg-project/issues/1182).
- `TestASheetFromBeforeTheWordStillLeavesTheCloak` passed before the code too: it pins a behaviour that must not change, so it was not seen failing — [https://github.com/KirkDiggler/rpg-project/issues/1182](https://github.com/KirkDiggler/rpg-project/issues/1182).

## Mistaken

- Mutation 1 was restored with `git checkout -- <file>` on an uncommitted tree and wiped the implementation; it was written again and committed before mutation 2 — [https://github.com/KirkDiggler/rpg-project/issues/1182](https://github.com/KirkDiggler/rpg-project/issues/1182).
- The gap on F4 (existing test pinning `Unique:false`) was fixed with one line in `lieutenants_cloak_test.go`, a listed file, and reported as a non-blocking gap — [comment](https://github.com/KirkDiggler/rpg-project/issues/1182#issuecomment-7183061653).

— cross-team agent, on behalf of KirkDiggler
