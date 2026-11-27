# Run — 2026-11-28 — NPC behaviour 3a a monster runs a described ability

Slice [rpg-project#1231](https://github.com/KirkDiggler/rpg-project/issues/1231) of story [rpg-project#1131](https://github.com/KirkDiggler/rpg-project/issues/1131). Base: `rpg-toolkit` `main` at [`a097088a`](https://github.com/KirkDiggler/rpg-toolkit/commit/a097088a).

## Shipped

- Seven tests through `Resolve` prove a monster built from a stat block's `lets` runs Second Wind as its own actor and payer: paid at the door, healed, reported as a player's heal, sheet in `DirtyMonsters` — [rpg-project#1231](https://github.com/KirkDiggler/rpg-project/issues/1231).
- The pin of `rulebooks/dnd5e` from v0.240.0 to v0.243.0; `go mod tidy` moved only that line and its two go.sum lines — [rpg-project#1231](https://github.com/KirkDiggler/rpg-project/issues/1231).
- ARCHITECTURE.md, AGENTS.md and the comment on the refusal in `Start` say which door a monster's described ability takes — [rpg-project#1231](https://github.com/KirkDiggler/rpg-project/issues/1231).

## Decided

- No production code changed: the proof held, so effect.go and healing.go are untouched and activation.go changed in one comment only — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1231#issuecomment-7226780763).
- The AGENTS.md words are one row in the "Where does this go?" table beside the driven-arm row, because that table is where the file says which door a thing takes — [rpg-project#1231](https://github.com/KirkDiggler/rpg-project/issues/1231).

## Found

- F1 holds: origin/main pinned `v0.240.0` — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1231#issuecomment-7226780763).
- F2 holds: `Grants` and `GrantedAbility` at granted.go:33 and :50, `Lets json.RawMessage` at template.go:82 and :309 — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1231#issuecomment-7226780763).
- F3 holds: `newEffect` at effect.go:38 and its `NewActivation` call at :66 — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1231#issuecomment-7226780763).
- F4 holds: `if m.cast != nil` at activation.go:575, the monster refusal at :589, the branch first — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1231#issuecomment-7226780763).
- F5 holds: `TestAMonsterAsPayerOfAnEffectsCostIsRefused` passes unedited, a wolf having no kit — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1231#issuecomment-7226780763).
- F6 holds: build, vet and the whole suite pass at the old pin and at v0.243.0 — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1231#issuecomment-7226780763).
- F7: `Input.Validate` (resolve.go:169) looks at no actor kind; `NewAction` (action.go) and `newEffect` refuse only an empty actor, another target, no roller and a body that is not a self heal; nothing refuses a monster as `AttackerID` — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1231#issuecomment-7226780763).
- Worked as read: the door charges the monster's bonus action and its one use together; `startCast` delivers before the monster refusal; `healable` admits a monster; the outcome has one `healing-applied` effect naming it — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1231#issuecomment-7226780763).
- Test 3 (full hit points): the action runs and is paid. The outcome has one `healing-applied` effect with Amount 0, Requested 7, Before 18, After 18; the bonus action and the use are spent — [rpg-project#1231](https://github.com/KirkDiggler/rpg-project/issues/1231).
- Test 4: the heal's calculation names the creature on the level modifier (label "Captain", source id the creature's id), and Second Wind with the creature's id on the dice — [rpg-project#1231](https://github.com/KirkDiggler/rpg-project/issues/1231).
- A monster's maximum from `4d8` is 18, the floored average of the whole notation, not 16 — [rpg-project#1231](https://github.com/KirkDiggler/rpg-project/issues/1231).
- Proof (a), no `Cost` sent: the tests for Test 1 (bonus action and use not spent), Test 2 (no `ErrCannotPay`), Test 3 and Test 5 fail — [rpg-project#1231](https://github.com/KirkDiggler/rpg-project/issues/1231).
- Proof (b): no production change was made, so there is none to undo; what made it work as read are effect.go:66 (`NewActivation` with `cast`), activation.go:575 (the cast branch before the refusal) and healing.go `healable` (admits a monster) — [rpg-project#1231](https://github.com/KirkDiggler/rpg-project/issues/1231).
- The suite passes before the pin (old v0.240.0) and at the final head — [rpg-project#1231](https://github.com/KirkDiggler/rpg-project/issues/1231).

## Mistaken

- The first run of the tests failed on the fixture: `dirtyMonster` in driven_activation_test.go looks up the goblin id only, and the captain's maximum was assumed to be 16; both were fixed in the new file, no existing test was edited — [rpg-project#1231](https://github.com/KirkDiggler/rpg-project/issues/1231).
