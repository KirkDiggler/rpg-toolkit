# Run — 2026-11-28 — NPC behaviour 3c-0 a stored sheet says what its creature can use

Slice [rpg-project#1236](https://github.com/KirkDiggler/rpg-project/issues/1236) of story [rpg-project#1131](https://github.com/KirkDiggler/rpg-project/issues/1131). Base: `rpg-toolkit` `main` at [`7f71c972`](https://github.com/KirkDiggler/rpg-toolkit/commit/7f71c972).

## Shipped

- `monster.AbilitiesOf`: from a stored sheet, each grant's author word, grant name, name, the gate's description and whether the sheet can pay now; no caller yet and nothing in play changes — [rpg-project#1236](https://github.com/KirkDiggler/rpg-project/issues/1236).
- `GrantedAbility` and `AbilitiesOf` build a description by one unexported `describeGrant`; refusal words are unchanged and `granted_test.go` passes unedited — [rpg-project#1236](https://github.com/KirkDiggler/rpg-project/issues/1236).

## Decided

- The ledger over a stored sheet is a small unexported read-only type, `sheetLedger`, over `*Data`; its four reads call `accountOf` and `poolNamedIn`, moved out of `ledger.go` and called by `*Monster` too, and its four spends do nothing because `combat.CanPay` never calls them — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1236#issuecomment-7227371181).
- `Word` is the one in `features.NamedAbilities()` whose ref is the grant's, never the grant's name with its hyphen replaced; a grant with no word is refused by name — [rpg-project#1236](https://github.com/KirkDiggler/rpg-project/issues/1236).
- `Name` is the definition's own name: "Second Wind" — [rpg-project#1236](https://github.com/KirkDiggler/rpg-project/issues/1236).
- The README had no kit section, so the reading is described in a new one after "Construction and loading" — [rpg-project#1236](https://github.com/KirkDiggler/rpg-project/issues/1236).

## Found

- F1 to F6 hold at `7f71c972`: tag `rulebooks/dnd5e/v0.243.0`; `Grants` 33 and `GrantedAbility` 50; `Kit` 85, `Left` 88, `SlotsLeft` 126; `CanPay` 81 and `Ledger` 27; `DescribeFromRef` 53, `NamedAbility` 151, `NamedAbilities` 160; `CostOfDash` 148 — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1236#issuecomment-7227371181).
- F8: the grep prints `resolution/depart.go:217 cloneMonsterData` (copies through the persisted form, a separate module) and `session/monster_turn_test.go:700 cloneMonsterView` (a session view helper); the slice adds no field, so neither changes — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1236#issuecomment-7227371181).
- The rules have one named ability ("second wind"), so the sort is asserted over one grant — [rpg-project#1236](https://github.com/KirkDiggler/rpg-project/issues/1236).
- The whole module's tests pass before the change (34.6 s) and at the final head (24.5 s), `golangci-lint run ./...` prints 0 issues — [rpg-project#1236](https://github.com/KirkDiggler/rpg-project/issues/1236).
- Proof (a), `Payable` always true: `TestPayableFollowsTheSheet` and `TestPayableIsTheGatesOwnAnswer` fail on "bonus action spent" and "use spent" (expected false, actual true) — [rpg-project#1236](https://github.com/KirkDiggler/rpg-project/issues/1236).
- Proof (b), `Word` from the grant's name: `TestAStoredSheetSaysWhatItsCreatureCanUse` fails, expected "second wind", actual "second-wind" — [rpg-project#1236](https://github.com/KirkDiggler/rpg-project/issues/1236).
- Proof (c), the ledger's `SlotsLeft` reports a full turn: both Payable tests fail on "bonus action spent" (expected false, actual true) — [rpg-project#1236](https://github.com/KirkDiggler/rpg-project/issues/1236).

## Mistaken

- The first form of proof (a) replaced `Payable` with `true` and left `ledger` unused, so the package did not build and proved nothing; it was redone as `CanPay(...) || true` and then failed as above — [rpg-project#1236](https://github.com/KirkDiggler/rpg-project/issues/1236).
