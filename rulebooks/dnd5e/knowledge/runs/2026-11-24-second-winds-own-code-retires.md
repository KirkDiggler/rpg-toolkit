# Run — 2026-11-24 — one way to pay for an ability S5 second wind's own activate code retires

processed: 2026-11-25T190427Z

Slice [rpg-project#1149](https://github.com/KirkDiggler/rpg-project/issues/1149) of story [rpg-project#1131](https://github.com/KirkDiggler/rpg-project/issues/1131). Base: `rpg-toolkit` `main` at [`8d5123f1`](https://github.com/KirkDiggler/rpg-toolkit/commit/8d5123f14c12a3379489cfe80da770d19e0c8b34).

## Shipped

- `SecondWind.Activate` only refuses, with `features.ErrUsedFromItsDescription`; the spend, the roll, the hand-built calculation and the published heal are deleted — [rpg-project#1149](https://github.com/KirkDiggler/rpg-project/issues/1149).
- `character.activateFeature` refuses a `features.Described` feature before it reads or takes the turn; features with no description take today's path — [rpg-project#1149](https://github.com/KirkDiggler/rpg-project/issues/1149).

## Decided

- The refusal is wrapped with the ability's name in `Activate` and returned bare as the output's `Error` text from `activateFeature`, so the sentence a player reads is exactly the error's message — [design 1 and 2](https://github.com/KirkDiggler/rpg-project/issues/1149).
- The dead-owner test now asks `CanActivate`, which still refuses a dead owner with `CodeInvalidState` before anything is spent or rolled; `Activate` no longer reaches that check — [allowed edits](https://github.com/KirkDiggler/rpg-project/issues/1149).
- The stand-in owners in `features` and `integration` pay through a small ledger of one fresh bonus action and the owner's pool, in a twin of the `character` helper, because neither can build a `Character` — [allowed edits](https://github.com/KirkDiggler/rpg-project/issues/1149).
- `TestAFighterAdvancedToTwoHealsOneD10PlusTwo` resolves the description built after advancing, with no payment, because that sheet is not in a fight — [allowed edits](https://github.com/KirkDiggler/rpg-project/issues/1149).

## Found

- Rests on: F1 to F6 hold at `8d5123f1`; F6 counted 19 call sites that activate Second Wind (features 8, integration 5, character 6) — [comment](https://github.com/KirkDiggler/rpg-project/issues/1149#issuecomment-7122556330).
- Suite before any change: every package `ok` — [comment](https://github.com/KirkDiggler/rpg-project/issues/1149#issuecomment-7122556330).
- Before the change the two refusal tests failed with `Success` true, an empty error, the bonus action and pool spent and one healing event published, and the direct call returned nil; the description and Rage tests already passed — [issue](https://github.com/KirkDiggler/rpg-project/issues/1149).
- Proofs: refusal after `consumeActionType` fails the refusal test; refusing every feature fails the Rage test; an `Activate` that succeeds again fails the direct-call test (a bare `return nil`, not the old body) — [issue](https://github.com/KirkDiggler/rpg-project/issues/1149).
- Existing tests: 5 removed in features (own roll), 1 removed in character (slice 2 agree test) and the bridge suite's one test (own roll); 2 changed in character pool, 1 in sheet facts, 2 in features (spend by description, dead owner), 4 in integration (by description) — [issue](https://github.com/KirkDiggler/rpg-project/issues/1149).
- Final head suite, race run and linter: green, 0 issues — [issue](https://github.com/KirkDiggler/rpg-project/issues/1149).

## Mistaken

- A test file was truncated by opening it for write before reading it in one script; `git checkout` restored it and the edit was redone — [issue](https://github.com/KirkDiggler/rpg-project/issues/1149).
