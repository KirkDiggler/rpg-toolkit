# Run — 2026-11-12 — Data-authored effect compositions

Design preparation for [rpg-project#703](https://github.com/KirkDiggler/rpg-project/issues/703), with item consumer [#850](https://github.com/KirkDiggler/rpg-project/issues/850). Base: toolkit main at [d7069af9](https://github.com/KirkDiggler/rpg-toolkit/commit/d7069af9). Implementation slices: provider [#946](https://github.com/KirkDiggler/rpg-project/issues/946), item cookbook [#947](https://github.com/KirkDiggler/rpg-project/issues/947).

## Shipped

- Documentation proposal only: a current rulebook-effects package guide, fixture-driven brief, detailed walkthrough/design and illustrative input library; no runtime or fixture implementation claimed — [walkthrough](../../docs/ideas/data-authored-effects/README.md).

## Decided

- KirkDiggler selected independently configurable benefits and maintenance: a ring maintains another recipient's Rage benefits through the user's concentration instead of recipient activity, while a scroll can use recipient attack/take-damage upkeep; the future studio authors the same data as the tests — [direction](https://github.com/KirkDiggler/rpg-project/issues/703#issuecomment-6831644739).

- At the shape checkpoint, KirkDiggler approved the component boundary; concrete authoring/frame contracts and explicit example policies remained open R4/R5 proposals, not silently attributed to that approval — [shape approval](https://github.com/KirkDiggler/rpg-toolkit/pull/2399#issuecomment-6832622204).

- KirkDiggler accepted the examples and requested the implementation plan; R4/R5 are settled and the current turn is planning-only, not runtime or release authorization — [planning direction](https://github.com/KirkDiggler/rpg-toolkit/pull/2399#issuecomment-6833714502).

## Found

- Damage, resistance and save/check advantage already have reusable output meanings, but only resistance has a payload part binding; Rage activity maintenance remains in its named recipe — [package guide](../../rulebooks/dnd5e/effects/README.md#the-vocabulary).
- ApplicationSpec contains concrete participants, the existing item example uses a named Go resolver, and target-held facts carry only ref/source identity; reusable authoring and configured target-side information require explicit contracts — [measured brief](https://github.com/KirkDiggler/rpg-project/issues/703#issuecomment-6831644739).
- Existing root effects/conditions and item-example tests pass at the measured base; those baselines do not establish the proposed fixture capability — [evidence](https://github.com/KirkDiggler/rpg-project/issues/703#issuecomment-6831644739).

- CurrentOutputs retains pre-terminal observations while Snapshot already reports terminal state; configured-effect projection must follow the former rather than infer absence from the latter — [projection contract](../../docs/ideas/data-authored-effects/design.md#information-and-execution).

- The checked plan assigns nine tasks to two module-scoped delivery slices, including typed capability/projection contracts and explicit native exclusions — [coverage and seam checks](../../docs/ideas/data-authored-effects/plan.md#visible-plan-checks).

## Mistaken

- The initial illustrative JSON used the display spelling STR; checking the canonical ability constant corrected it to str before publication. The authored examples use existing vocabulary spellings, not new aliases — [examples](../../docs/ideas/data-authored-effects/examples.md#one-library-five-applications).
