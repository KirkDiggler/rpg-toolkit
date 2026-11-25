# Run — 2026-11-21 — Status effect powers S6 initiative rolls in the session

processed: 2026-11-25T190427Z

Slice [rpg-project#1050](https://github.com/KirkDiggler/rpg-project/issues/1050) of story [rpg-project#1036](https://github.com/KirkDiggler/rpg-project/issues/1036). Base: `rpg-toolkit` `main` at [`771116fb`](https://github.com/KirkDiggler/rpg-toolkit/commit/771116fb).

One line per entry. Every line ends with a link.

## Shipped

- The session rolls initiative through `initiative.RollOne` and hands each roll to the composition, so the debug record's `bubble-formed` entry carries `rolls`; the act stream's `FightFormed` carries `Rolls`; pins `rulebooks/dnd5e v0.224.0`, `encounter v0.237.0` — [rpg-toolkit#2502](https://github.com/KirkDiggler/rpg-toolkit/pull/2502).

## Decided

- The rolls reach the story log through the record entry and `clockPieces`, the path every other clock entry takes, and not through the verb's `Formed` return — [rpg-project#1050](https://github.com/KirkDiggler/rpg-project/issues/1050).
- `Formed` (the verb's own report) is left with order and surprise only — [rpg-project#1050](https://github.com/KirkDiggler/rpg-project/issues/1050).
- Existing fixtures' illegal initiative faces become 1, per Amendment 1 (every member moves up by the same 1, so no order changes) — [Amendment 1](https://github.com/KirkDiggler/rpg-project/issues/1050#issuecomment-7051755528).
- The session reads `InitiativeContribution` (classified in `conditionReaders`) and names no other rules type for initiative — [Amendment 2](https://github.com/KirkDiggler/rpg-project/issues/1050#issuecomment-7051887957).

## Found

- The session's fixtures scripted an initiative d20 face of 0, which the old order-only roll accepted and the rulebook's recorded roll refuses — [gap](https://github.com/KirkDiggler/rpg-project/issues/1050#issuecomment-7051742133).
- The module's `conditionReaders` pin refuses the rules answer type the issue had the session return, so `TestSessionConstructsNoCheck` fails — [gap](https://github.com/KirkDiggler/rpg-project/issues/1050#issuecomment-7051862600).
- The three suites of step 9 run in about 0.15 s together; the whole module's package run takes about 58 s — [rpg-toolkit#2502](https://github.com/KirkDiggler/rpg-toolkit/pull/2502).

## Mistaken

- The slice issue assumed the swap to `RollOne` changed nothing for existing fixtures; the builder's gap report caught it — [gap](https://github.com/KirkDiggler/rpg-project/issues/1050#issuecomment-7051742133).
- The slice issue named a rules type the module's own pin refuses; the builder's gap report caught it — [gap](https://github.com/KirkDiggler/rpg-project/issues/1050#issuecomment-7051862600).
