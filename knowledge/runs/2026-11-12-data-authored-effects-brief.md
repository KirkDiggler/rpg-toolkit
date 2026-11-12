# Run — 2026-11-12 — Data-authored effect compositions

Design preparation for [rpg-project#703](https://github.com/KirkDiggler/rpg-project/issues/703), with item consumer [#850](https://github.com/KirkDiggler/rpg-project/issues/850). Base: toolkit main at [d7069af9](https://github.com/KirkDiggler/rpg-toolkit/commit/d7069af9). Implementation: rulebook and its cookbook [#946](https://github.com/KirkDiggler/rpg-project/issues/946); downstream item adoption [#947](https://github.com/KirkDiggler/rpg-project/issues/947) is deferred and separate.

## Shipped

- Documentation proposal only: a current rulebook-effects package guide, fixture-driven brief, detailed walkthrough/design and illustrative input library; no runtime or fixture implementation claimed — [walkthrough](../../docs/ideas/data-authored-effects/README.md).

## Decided

- KirkDiggler selected independently configurable benefits and maintenance: a ring maintains another recipient's Rage benefits through the user's concentration instead of recipient activity, while a scroll can use recipient attack/take-damage upkeep; the future studio authors the same data as the tests — [direction](https://github.com/KirkDiggler/rpg-project/issues/703#issuecomment-6831644739).

- At the shape checkpoint, KirkDiggler approved the component boundary; concrete authoring/frame contracts and explicit example policies remained open R4/R5 proposals, not silently attributed to that approval — [shape approval](https://github.com/KirkDiggler/rpg-toolkit/pull/2399#issuecomment-6832622204).

- KirkDiggler accepted the examples and requested the implementation plan; R4/R5 are settled and the current turn is planning-only, not runtime or release authorization — [planning direction](https://github.com/KirkDiggler/rpg-toolkit/pull/2399#issuecomment-6833714502).

- KirkDiggler selected the rulebook as the primary reusable and teaching surface: R6 moves the canonical data and public-API examples beside the production rulebook code, with only a small test source; examples/items adoption is no longer a prerequisite — [correction](https://github.com/KirkDiggler/rpg-toolkit/pull/2399#issuecomment-6835450799).

## Found

- Damage, resistance and save/check advantage already have reusable output meanings, but only resistance has a payload part binding; Rage activity maintenance remains in its named recipe — [package guide](../../rulebooks/dnd5e/effects/README.md#the-vocabulary).
- ApplicationSpec contains concrete participants, the existing item example uses a named Go resolver, and target-held facts carry only ref/source identity; reusable authoring and configured target-side information require explicit contracts — [measured brief](https://github.com/KirkDiggler/rpg-project/issues/703#issuecomment-6831644739).
- Existing root effects/conditions and item-example tests pass at the measured base; those baselines do not establish the proposed fixture capability — [evidence](https://github.com/KirkDiggler/rpg-project/issues/703#issuecomment-6831644739).

- CurrentOutputs retains pre-terminal observations while Snapshot already reports terminal state; configured-effect projection must follow the former rather than infer absence from the latter — [projection contract](../../docs/ideas/data-authored-effects/design.md#information-and-execution).

- The revised checked plan assigns nine tasks to one root-module deliverable, retaining typed capability/projection contracts and explicit native exclusions — [coverage and seam checks](../../docs/ideas/data-authored-effects/plan.md#visible-plan-checks).

## Mistaken

- The first plan put the reusable implementation in the rulebook but made examples/items the primary cookbook and integration deliverable. That displaced the operator's goal of getting the rulebook package in order; R6 moves the teaching and proof beside it rather than defending the extra consumer step — [correction](https://github.com/KirkDiggler/rpg-toolkit/pull/2399#issuecomment-6835450799).

- The initial illustrative JSON used the display spelling STR; checking the canonical ability constant corrected it to str before publication. The authored examples use existing vocabulary spellings, not new aliases — [examples](../../docs/ideas/data-authored-effects/examples.md#one-library-five-applications).
