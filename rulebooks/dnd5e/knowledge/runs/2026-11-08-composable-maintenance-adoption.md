# Run — 2026-11-08 — Composable conditions S14 rulebook adoption

Slice [rpg-project#904](https://github.com/KirkDiggler/rpg-project/issues/904), story [rpg-project#703](https://github.com/KirkDiggler/rpg-project/issues/703). Base [d4033752](https://github.com/KirkDiggler/rpg-toolkit/commit/d4033752); [approved plan](https://github.com/KirkDiggler/rpg-toolkit/blob/design/bless-bane-composition/docs/ideas/composable-maintenance/plan.md).

## Shipped

- Initial source checkpoint adopts provider cc0c8dee and the single typed amount contract for existing numeric meanings; new maintenance/application work T5–T8 is not complete or released — [provider #2327](https://github.com/KirkDiggler/rpg-toolkit/pull/2327).

## Decided

- Existing damage bonuses explicitly require scalar results; information uses nil for absent amounts. This is current numeric semantics, not a compatibility evaluator or a hidden die roll — [amount contract](https://github.com/KirkDiggler/rpg-toolkit/blob/design/bless-bane-composition/docs/ideas/composable-maintenance/README.md#describing-amounts-and-query-only-outputs).

## Found

- Adopting the pushed provider first exposes four compile failures in the integer-only root evaluator/vocabulary; explicit scalar producers/consumers restore the existing conditions/effects tests without changing Raging gameplay — [S14](https://github.com/KirkDiggler/rpg-project/issues/904).

## Mistaken

- None observed in this initial adoption checkpoint; the compile failures are expected evidence of consuming the changed real provider instead of a local replacement — [S14](https://github.com/KirkDiggler/rpg-project/issues/904).
