# Run — 2026-11-26 — item story S: a looted item lands in the inventory

Slice [rpg-project#1169](https://github.com/KirkDiggler/rpg-project/issues/1169) of story [rpg-project#850](https://github.com/KirkDiggler/rpg-project/issues/850). Base: `rpg-toolkit` `main` at [`ce7eab72`](https://github.com/KirkDiggler/rpg-toolkit/commit/ce7eab72).

## Shipped

- Pins: `rulebooks/dnd5e` v0.233.0 to v0.234.0, `rulebooks/dnd5e/encounter` v0.255.0 to v0.258.0 (the issue said v0.257.0; Amendment 3 moved it); resolution stays v0.82.0 — [Amendment 3](https://github.com/KirkDiggler/rpg-project/issues/1169#issuecomment-7173456496).
- `Loot` keeps the encounter's output, reads each taken item with `magicitems.Compile`, makes it with an id of its own and adds it to the looter's saved sheet before the commit; nothing is worn — [issue Design 3](https://github.com/KirkDiggler/rpg-project/issues/1169).
- `BeatTaken` is record-only in `act_family.go`; no act piece is produced for it — [issue Design 2](https://github.com/KirkDiggler/rpg-project/issues/1169).
- `CheckContent` reads every `ItemGrant` the site carries (creature equipment or prop item) once per item id into `CheckContentOutput.Items`; launch refuses it as `ErrUnknownContent` through `refuseLaunchContent` (renamed from `refuseLaunchStatuses`) — [issue Design 6](https://github.com/KirkDiggler/rpg-project/issues/1169).
- `launch.go` hands a placed creature's `placement.Equipment` to `join.Equipment` — [Amendment 2](https://github.com/KirkDiggler/rpg-project/issues/1169#issuecomment-7173019613).
- New site `testdata/lieutenants-cloak-walk.yaml`, copied from `second-wind-walk.yaml` (the template renamed Lieutenant, the `customItems` block, `equipment: [lieutenants-cloak]` on the guard), played by `TestTheLieutenantsCloakIsLootedWornAndUsed` — [issue Design 7](https://github.com/KirkDiggler/rpg-project/issues/1169).

## Decided

- The looter's sheet is saved on every loot by a player, so `Saved.Written` is `character:alice, encounter:sess` whether or not the body carried anything; `TestLootOnTheCaptainRevealsTheVaultToTheLooterAlone` was the only existing test that pinned a loot's `Written` and its expectation changed (the only edit to an existing test) — [Amendment 1](https://github.com/KirkDiggler/rpg-project/issues/1169#issuecomment-7172954827).
- A looter with no sheet (not a player) takes nothing from the encounter; `Loot` loads and saves no sheet for it and saves the encounter alone, as before. A missing sheet with something taken is an error. Pinned at `deliverLoot`, not through `Loot`, because no existing fixture has a monster looting a downed body in free roam — [gap comment](https://github.com/KirkDiggler/rpg-project/issues/1169#issuecomment-7172943618).
- The item's id is the author's id, a `-` and the first 8 hex digits of SHA-256 over session, body and item id joined by NUL — [issue Design 4](https://github.com/KirkDiggler/rpg-project/issues/1169).

## Found

- `dungeonspec.MonsterPlacement.Equipment` was never read by the session: launch did not hand it over, so no launched body carried an item — [gap](https://github.com/KirkDiggler/rpg-project/issues/1169#issuecomment-7173009098).
- `encounter.JoinInput` (v0.257.0) had no `Equipment` field; it exists at v0.258.0 — [gap](https://github.com/KirkDiggler/rpg-project/issues/1169#issuecomment-7173034151).
- Places the session builds a member input from a compiled `MonsterPlacement`: only `launch.go` (the monster join, changed). `launch.go:210` builds the party's join and `write.go:516` (`place`, behind `Join` and `PlaceNPC`) builds one from a caller's plain `holds []string` (the `Holds` at `write.go:552` is `intelIDs(holds)`), never from a placement, so equipment does not belong there; unchanged — [gap](https://github.com/KirkDiggler/rpg-project/issues/1169#issuecomment-7173034151).
- F5: `Refusals` is read in code only by launch (`statuses.go` `refuseLaunchStatuses`) and in tests by `castle_kitchen_rewards_test.go` and `hold_status_test.go` — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1169#issuecomment-7172848415).
- F7: `LootOutput` is built in one place, `loot.go` `return &LootOutput{Saved: report, Delivery: delivery}`; no field was added — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1169#issuecomment-7172848415).
- F6: the loot test followed is `holdingsscenes_test.go` (`s.mgr.Loot(ctx, &session.LootInput{...})`); the fighter downs the Lieutenant on turn one with a critical hit on a guard set to 1 hit point, as `second_wind_walk_test.go` does — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1169#issuecomment-7172848415).
- The cloak is equipped after the fight, in free roam, through `Equip` into the cloak slot, because a fight refuses changing a worn slot (`ErrArmorInFight`) — [worn_item_row_test.go Test 8](https://github.com/KirkDiggler/rpg-toolkit/blob/ce7eab72/rulebooks/dnd5e/session/worn_item_row_test.go).
- A placed prop's `item:` reaches the session's check with no help: the compiled site carries it and `CheckContent` judges it with no creature naming it (pinned); nothing takes a prop yet, and whether it reaches a launched encounter was not checked — [issue Not in this slice](https://github.com/KirkDiggler/rpg-project/issues/1169).
- On the pins alone, the whole-module suite failed only `TestEveryBeatHasAFamily` ("Should be empty, but was [BeatTaken (taken)]") — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1169#issuecomment-7172848415).
- Test 5 compares the whole `LootOutput` (`Saved` and `Delivery`) of a body with the cloak and of the same site with the `equipment` line removed; the session id and body id are the same in both, so nothing is excluded — [issue Design 5](https://github.com/KirkDiggler/rpg-project/issues/1169).
- The three proofs: discarding the taken items fails `TestTheLieutenantsCloakIsLootedWornAndUsed`; saving the sheet only when something was taken fails `TestLootAnswersTheSameWhateverTheBodyCarried`; skipping items in `CheckContent` fails `TestCheckContentJudgesEveryCustomItem` — [issue](https://github.com/KirkDiggler/rpg-project/issues/1169).
- At the final head the whole-module suite passes in about 60 seconds, as do build, vet, the race run on the named tests and the linter — [issue](https://github.com/KirkDiggler/rpg-project/issues/1169).

## Mistaken

- The delivery was written before its tests, against the order the issue sets; the three mutation proofs were run afterwards to show the tests bite — [Amendment 1](https://github.com/KirkDiggler/rpg-project/issues/1169#issuecomment-7172954827).
- The brief said no existing test would change and missed that launch never handed over equipment; both surfaced as gaps and were ruled — [Amendment 2](https://github.com/KirkDiggler/rpg-project/issues/1169#issuecomment-7173019613).
