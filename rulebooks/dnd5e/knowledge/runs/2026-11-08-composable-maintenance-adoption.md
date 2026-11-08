# Run — 2026-11-08 — Composable conditions S14 rulebook adoption

Slice [rpg-project#904](https://github.com/KirkDiggler/rpg-project/issues/904), story [rpg-project#703](https://github.com/KirkDiggler/rpg-project/issues/703). Base [d4033752](https://github.com/KirkDiggler/rpg-toolkit/commit/d4033752); [approved plan](https://github.com/KirkDiggler/rpg-toolkit/blob/design/bless-bane-composition/docs/ideas/composable-maintenance/plan.md).

## Shipped

- Initial source checkpoint adopts provider cc0c8dee and the single typed amount contract for existing numeric meanings; application/codec/native work T6–T8 is not complete or released — [provider #2327](https://github.com/KirkDiggler/rpg-toolkit/pull/2327).

- T5 source supplies reusable concentration, duration, end-on, until-empty, resistance and query-only roll-modifier parts in the one kind table; native callers are not migrated by this vocabulary checkpoint — [T5 contract](https://github.com/KirkDiggler/rpg-toolkit/blob/design/bless-bane-composition/docs/ideas/composable-maintenance/plan.md#t5--rulebook-part-meanings-and-typed-contribution-adoption).

## Decided

- A reactive concentration output checks the provider's CanReact permission before requesting a save, separately from contributions still available during end reporting; this consumes provider eb5ffb63 rather than copying lifecycle state — [provider #2327](https://github.com/KirkDiggler/rpg-toolkit/pull/2327).
- RollQuery names the actual roll recipient, while a save-shaped outgoing action does not make its caster a saver; signed dice, selection group and source come from the same kind assessment used by information — [S14](https://github.com/KirkDiggler/rpg-project/issues/904).
- Existing damage bonuses explicitly require scalar results; information uses nil for absent amounts. This is current numeric semantics, not a compatibility evaluator or a hidden die roll — [amount contract](https://github.com/KirkDiggler/rpg-toolkit/blob/design/bless-bane-composition/docs/ideas/composable-maintenance/README.md#describing-amounts-and-query-only-outputs).

## Found

- Adopting the pushed provider first exposes four compile failures in the integer-only root evaluator/vocabulary; explicit scalar producers/consumers restore the existing conditions/effects tests without changing Raging gameplay — [S14](https://github.com/KirkDiggler/rpg-project/issues/904).

- New part APIs first fail by absence; resistance binding, nil-request attribution and payload inspection fail behaviorally before correction. Mutants dropping reaction permission, exact-owner qualification and subtraction fail the corresponding tests before exact restoration — [S14](https://github.com/KirkDiggler/rpg-project/issues/904).

## Mistaken

- An initial resistance fixture treated CreateFromRefOutput as its Condition; correcting the fixture precedes the meaningful unsupported-part RED. The first roll implementation also confused the single Answer.Roll with plural Effect rows; roll data now stays in the shared kind assessment without changing the information-row contract — [S14](https://github.com/KirkDiggler/rpg-project/issues/904).
