# Run — 2026-11-26 — item story S: a character owns one of a custom item

Slice [rpg-project#1177](https://github.com/KirkDiggler/rpg-project/issues/1177) of story [rpg-project#850](https://github.com/KirkDiggler/rpg-project/issues/850). Base: `rpg-toolkit` `main` at [`bb118c4e`](https://github.com/KirkDiggler/rpg-toolkit/commit/bb118c4e400a984b3e5fedfd4e02523b968b9c2e).

## Shipped

- A custom item's id on a sheet is the site's item (`siteItemID`), so a character owns at most one of it — [rpg-project#1177](https://github.com/KirkDiggler/rpg-project/issues/1177).
- `Manager.Loot` reads the looter's sheet before the loot and tells the encounter what to leave (`leaveFor` into `LootInput.Leave`); the sheet is loaded once and is the one delivered to and saved — [rpg-project#1177](https://github.com/KirkDiggler/rpg-project/issues/1177).
- Pin `rulebooks/dnd5e/encounter` v0.259.0 to v0.260.0 (`go.mod` and `go.sum` only) — [rpg-project#1177](https://github.com/KirkDiggler/rpg-project/issues/1177).

## Decided

- The function that makes `Leave` is `leaveFor(siteKey, sheet)` and the load is `loadLooter`; `deliverLoot` takes the loaded sheet — [rpg-project#1177 design 2 and 3](https://github.com/KirkDiggler/rpg-project/issues/1177).
- Nothing is migrated: an item under an old run-and-body id is another item, said in the comment at `siteItemID` — [rpg-project#1177 design 4](https://github.com/KirkDiggler/rpg-project/issues/1177).
- The second-body test site is a copy of the walk site with `guard-2` one cell south, down at the start, wearing the same cloak — [rpg-project#1177 test 3](https://github.com/KirkDiggler/rpg-project/issues/1177).

## Found

- F0 printed `github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter v0.259.0`; newest session tag `v0.213.0` — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1177#issuecomment-7178904275).
- F5 on the pin alone: `go build ./...` printed nothing; `go test ./... -count=1` passed both packages (session 59.99s, session-workbench 0.04s); no failures — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1177#issuecomment-7178904275).
- A launch is a first-admission long rest and refills the cloak to 1 of 1, so a second run cannot start at 0 of 1; Test 1 uses the cloak again in the second run before it loots — [gap comment](https://github.com/KirkDiggler/rpg-project/issues/1177#issuecomment-7178988100).
- A looter at range 1 of the walk site's second Lieutenant and of a second player's position is out of range; the new tests pass Range 3 — [gap comment](https://github.com/KirkDiggler/rpg-project/issues/1177#issuecomment-7178988100).
- go.mod diff: `rulebooks/dnd5e/encounter v0.259.0 => v0.260.0` — [rpg-project#1177](https://github.com/KirkDiggler/rpg-project/issues/1177).
- Existing tests changed: `TestLootedItemID` rewritten as `TestSiteItemID` (its subject changed); `TestDeliverTakenRefusesWhatTheRulesRefuse` takes a site key in place of a session id; `TestALooterWithNoSheetSavesNoSheet` loads through `loadLooter` and passes the sheet to `deliverLoot` (changed signature); `newCloakScene` in `lieutenants_cloak_test.go` now wraps `newCloakSceneOn` (a session id and a shared character store). `TestLootingTheSameBodyTwiceAddsNothing` needed no edit — [rpg-project#1177](https://github.com/KirkDiggler/rpg-project/issues/1177).
- Proof 1, no `Leave` passed: Tests 1, 2, 3 and 4 fail with `loot: item "lieutenants-cloak" taken from "guard": item "lieutenants-cloak-58e4ebf9" is already owned` (and the same for "guard-2"): `AddItem` refuses the second cloak, so the loot is refused — [rpg-project#1177](https://github.com/KirkDiggler/rpg-project/issues/1177).
- Proof 2, the session id back into the id: Test 1 fails with "exactly one cloak, with the id it had" (Not equal) and Test 2 with "alice left it on the body" — [rpg-project#1177](https://github.com/KirkDiggler/rpg-project/issues/1177).
- New test times with `-v`: TestSiteItemID 0.00s, TestLeaveIsDerivedFromTheSheetAlone 0.00s, TestACharacterWhoOwnsTheCloakDoesNotTakeAnotherInALaterRun 0.04s, TestWhatALooterLeftStaysForAnotherPlayer 0.04s, TestALooterOfTwoBodiesTakesOneCloak 0.03s, TestLootAnswersTheSameWhetherTheLooterTookOrLeftTheCloak 0.05s — [rpg-project#1177](https://github.com/KirkDiggler/rpg-project/issues/1177).
- Suite at the head: `go build`, `go vet`, `golangci-lint run ./...` clean; `go test ./... -count=1 -timeout 20m` passed both packages (session 60.71s) — [rpg-project#1177](https://github.com/KirkDiggler/rpg-project/issues/1177).

## Mistaken

- The first commit was refused by the hook's linter (nilnil on `loadLooter`); fixed before the commit landed — [rpg-project#1177](https://github.com/KirkDiggler/rpg-project/issues/1177).
