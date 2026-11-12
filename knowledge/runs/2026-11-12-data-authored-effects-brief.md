# Run — 2026-11-12 — Data-authored effect compositions

Design preparation for [rpg-project#703](https://github.com/KirkDiggler/rpg-project/issues/703), with item consumer [#850](https://github.com/KirkDiggler/rpg-project/issues/850). Base: toolkit main at [d7069af9](https://github.com/KirkDiggler/rpg-toolkit/commit/d7069af9). Implementation slices are not selected yet.

## Shipped

- Documentation proposal only: a current rulebook-effects package guide and fixture-driven composition brief; no runtime or fixture implementation claimed — [brief](../../docs/ideas/data-authored-effects/brief.md).

## Decided

- KirkDiggler selected independently configurable benefits and maintenance: a ring maintains another recipient's Rage benefits through the user's concentration instead of recipient activity, while a scroll can use recipient attack/take-damage upkeep; the future studio authors the same data as the tests — [direction](https://github.com/KirkDiggler/rpg-project/issues/703#issuecomment-6831644739).

## Found

- Damage, resistance and save/check advantage already have reusable output meanings, but only resistance has a payload part binding; Rage activity maintenance remains in its named recipe — [package guide](../../rulebooks/dnd5e/effects/README.md#the-vocabulary).
- ApplicationSpec contains concrete participants, the existing item example uses a named Go resolver, and target-held facts carry only ref/source identity; reusable authoring and configured target-side information require explicit contracts — [measured brief](https://github.com/KirkDiggler/rpg-project/issues/703#issuecomment-6831644739).
- Existing root effects/conditions and item-example tests pass at the measured base; those baselines do not establish the proposed fixture capability — [evidence](https://github.com/KirkDiggler/rpg-project/issues/703#issuecomment-6831644739).

## Mistaken

- None observed in the implementation; no runtime implementation is part of this change — [scope](../../docs/ideas/data-authored-effects/brief.md#non-goals).
