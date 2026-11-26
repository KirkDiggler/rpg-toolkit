# Run — 2026-11-26 — an NPC's turn is paid at the one gate S1a a monster's sheet answers the gate

Slice [rpg-project#1191](https://github.com/KirkDiggler/rpg-project/issues/1191) of story [rpg-project#1131](https://github.com/KirkDiggler/rpg-project/issues/1131). Base: `rpg-toolkit` `main` at [`9ccd3917`](https://github.com/KirkDiggler/rpg-toolkit/commit/9ccd3917).

One line per entry. Every line ends with a link.

## Shipped

- `monster.Data.Economy` replaces `ReactionSpent`; the sheet is a `combat.Ledger` (`monster/ledger.go`); `combat.ActionEconomy` carries JSON tags; its own turn start gives the turn back — [commit bb5a05c4](https://github.com/KirkDiggler/rpg-toolkit/commit/bb5a05c4).
- Six comments reworded to what is true after the slice (monster.go, gate.go, action_economy_ledger.go, combatant.go, events.go, character/ledger.go) — [commit bb5a05c4](https://github.com/KirkDiggler/rpg-toolkit/commit/bb5a05c4).

## Decided

- Nil `Economy` is a full turn, and `ToData` and `Load` copy the account; the copy is a private `copyEconomy` in `monster/ledger.go` because `combat/action_economy.go` was listed for tags and a comment only — [slice issue Design 2 and 7](https://github.com/KirkDiggler/rpg-project/issues/1191).
- A reaction is floored at zero in `SpendSlots`, as a character's is — [slice issue Design 4](https://github.com/KirkDiggler/rpg-project/issues/1191).
- An old blob with `"reaction_spent": true` loads as a full turn; pinned by `TestAnOldBlobWithTheRemovedFieldLoadsAsAFullTurn` — [slice issue Design 2](https://github.com/KirkDiggler/rpg-project/issues/1191).

## Found

- Rests on, as run: F1 printed `24:var _ Ledger = (*ActionEconomy)(nil)`; F2 listed only the files the issue names; F3 found `combat.ActionEconomy` held only in `character/action_economy.go` (built in code), `combatabilities/input.go:22` and `features/types.go:31` (both `json:"-"`) and no golden file; F4 printed nothing — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1191#issuecomment-7185729523).
- F5: `go test ./... -count=1 -timeout 20m` in `rulebooks/dnd5e` passed with no failing package before any change; the fresh goblin's saved JSON on `origin/main` is 1181 bytes and is pinned as `goblinBeforeTheSlice` — [commit bb5a05c4](https://github.com/KirkDiggler/rpg-toolkit/commit/bb5a05c4).
- Test 6's saved JSON after one bonus action is `"economy":{"actions_remaining":1,"bonus_actions_remaining":0,"reactions_remaining":1,"object_interactions_remaining":1}` (the free object interaction is seeded with the turn) — [commit bb5a05c4](https://github.com/KirkDiggler/rpg-toolkit/commit/bb5a05c4).
- Existing tests edited, each only to ask the same question the new way: `monster/load_test.go` (`TestASpentReactionSurvivesTheRoundTrip`, `TestAnOlderBlobWithNoMeterHasItsReaction`; plus the `combat` import) and `monstertraits/monster_carries_a_condition_test.go` (two assertions; plus the `coreCombat` import); `monster/monster_markdirty_test.go` needed no edit — [commit bb5a05c4](https://github.com/KirkDiggler/rpg-toolkit/commit/bb5a05c4).
- Proofs, each restored with `git checkout --`: (a) `ToData` hands out the live pointer: `TestTheSheetAndItsSavedFormShareNothing` fails; (b) `onTurnStart` leaves the account: `TestItsOwnTurnStartGivesTheTurnBackAndOnlyItsOwn` and `TestMonsterKeeperSuite` fail; (c) no `omitempty`: `TestAMonsterThatSpentNothingSavesNoAccount`, `TestAskingSpendsNothing` and Test 5 fail; (d) `SlotsLeft` creates the account: `TestAskingSpendsNothing`, `TestAProfileThatCannotBePaidWholeIsPaidNotAtAll`, Test 5, `TestASpendMarksTheSheetAndARefusalDoesNot` and the keeper suite fail — [commit bb5a05c4](https://github.com/KirkDiggler/rpg-toolkit/commit/bb5a05c4).
- Final head: `go build ./...`, `go vet ./...`, `golangci-lint run ./...` (0 issues) and `go test ./... -count=1 -timeout 20m` all pass — [commit bb5a05c4](https://github.com/KirkDiggler/rpg-toolkit/commit/bb5a05c4).

## Mistaken

- none observed — [slice issue](https://github.com/KirkDiggler/rpg-project/issues/1191).
