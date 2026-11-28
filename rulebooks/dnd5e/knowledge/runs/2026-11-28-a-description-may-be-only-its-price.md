# Run — 2026-11-28 — second ability 5b-1b a description may be only its price

Slice [rpg-project#1260](https://github.com/KirkDiggler/rpg-project/issues/1260) of story [rpg-project#1131](https://github.com/KirkDiggler/rpg-project/issues/1131). Base: `rpg-toolkit` `main` at [`4ed327b5`](https://github.com/KirkDiggler/rpg-toolkit/commit/4ed327b5).

## Shipped

- `Definition.OnlyItsPrice` and a `Validate` that accepts no profile when the cost gives a turn slot, in `rulebooks/dnd5e/combat/actions`; becomes `rulebooks/dnd5e/v0.246.0` unless another rules slice merges first — [rpg-project#1260](https://github.com/KirkDiggler/rpg-project/issues/1260).

## Decided

- Zero profiles with nothing given is refused with `action must populate exactly one profile, or give a turn slot as its price`; two or more keep the old sentence unchanged — [rpg-project#1260](https://github.com/KirkDiggler/rpg-project/issues/1260).
- The profile rule runs before the cost's own check, and a gift counts toward it, so a malformed gift is refused by the cost's sentence (`action cost is invalid: ... slot grant keyed to ...`), which is what an author should read — [rpg-project#1260](https://github.com/KirkDiggler/rpg-project/issues/1260).
- A gift that charges nothing is valid and only its price; whether it may be run for nothing is the runner's rule — [rpg-project#1260](https://github.com/KirkDiggler/rpg-project/issues/1260).

## Found

- F3: every pin of `exactly one profile` is a substring match or a comment (`definition_test.go:109`, `cast_test.go:447,456`, `sequence_test.go:58`, `session/declaration_id_test.go:309` is a comment in another module), so no existing test was edited — [F3 comment](https://github.com/KirkDiggler/rpg-project/issues/1260#issuecomment-7231077076).
- F4: `monster.AddAction` (`monster/monster.go:571`) takes any caller-supplied definition and now accepts an only-its-price one; the weapon paths (`monster/weapons.go:99`, `weaponattack.go:195`, `off_hand_attack.go:60`) always carry an Attack and cannot reach it; `Describe` accepts it with empty facts — [F4 comment](https://github.com/KirkDiggler/rpg-project/issues/1260#issuecomment-7231077076).
- Proof (a), accepting zero profiles whatever the cost, failed `TestADescriptionWithNothingGivenIsRefused` (three cases: "An error is expected but got nil"), plus the existing `TestDefinitionValidation/no_profile` and `TestCastProfileSuite/TestADefinitionWithNoProfileIsStillRefused` — [rpg-project#1260](https://github.com/KirkDiggler/rpg-project/issues/1260).
- Proof (b), `OnlyItsPrice` ignoring the profiles, failed `TestProfilesAreAsTheyWere` (two cases: "Should be false") — [rpg-project#1260](https://github.com/KirkDiggler/rpg-project/issues/1260).
- Before the change, with a stub `OnlyItsPrice` returning false, the new tests failed on real assertions (unexpected error on the valid cases; old sentence missing the new words; the cost's sentence absent); the two-profile case passed because it is unchanged — [rpg-project#1260](https://github.com/KirkDiggler/rpg-project/issues/1260).
- `go build`, `go vet`, `go test ./... -count=1` pass before and at the final head; `golangci-lint run ./...` prints 0 issues — [rpg-project#1260](https://github.com/KirkDiggler/rpg-project/issues/1260).

## Mistaken

- The first mutation for proof (b) was a sed that broke the syntax and proved nothing; it was restored and redone as `return true && ...` — [rpg-project#1260](https://github.com/KirkDiggler/rpg-project/issues/1260).
