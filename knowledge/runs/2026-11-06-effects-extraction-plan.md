# Run — 2026-11-06 — Items-first effects extraction planning

Slice [rpg-project#705](https://github.com/KirkDiggler/rpg-project/issues/705) under [story #703](https://github.com/KirkDiggler/rpg-project/issues/703), consuming [item S1](https://github.com/KirkDiggler/rpg-project/issues/862) and [Rage/loot S2](https://github.com/KirkDiggler/rpg-project/issues/873).

## Shipped

- The provider source moves to mechanics/effects with machine inside the same module and Compose(ComposeInput) as the composition entry; no tag/release or merge is claimed — [module](../../mechanics/effects/README.md).

- Both actual item consumers now use the pushed provider pseudo-version: S1 7d7ef342 and S2 7dadbb64 pass full-module race tests, examples, vet and lint; no old effects dependency remains — [implementation evidence](../../docs/ideas/item-effect-composition/implementation.md).

## Decided

- The operator cleared toolkit#2288 and project#878 for merge; they now provide the canonical walkthrough and record/skill map that this plan derives from — [documentation work #876](https://github.com/KirkDiggler/rpg-project/issues/876).
- KirkDiggler settled H1 on 2026-11-06: mechanics/effects, with machine inside the release unit; Compose(ComposeInput) names composition while machine.New(Config) remains construction. The first top-level-home proposal is retained in the plan's reasoning — [settled choices](../../docs/ideas/item-effect-composition/plan.md#scope-and-settled-choices).
- Keep this cut mechanical and items-first; rulebook vocabulary, condition factories/migration and editor descriptors remain separate contracts rather than hidden prerequisites — [scope](../../docs/ideas/item-effect-composition/plan.md#scope-and-settled-choices).

## Found

- The live Go-import census places provider imports in examples/effects and consumer imports in examples/items; historical origin notes and pinned run records also name the example path but are not dependencies — [inspected baseline and migration tasks](../../docs/ideas/item-effect-composition/plan.md).
- The chosen path has published tags through v0.2.1, an ancestor of main; its replacement must receive CI's next pre-v1 version, not a fabricated first tag. CI discovers only the surviving module after this move — [release task](../../docs/ideas/item-effect-composition/plan.md#task-3--releaseadoption-evidence-only-with-the-required-authority).
- All 25 moved Go files have identical ASTs after normalizing only import prefixes and the agreed Compose/ComposeInput rename; comments/positions are excluded. Provider race tests, example, vet, lint and benchmark smoke pass — [provider tests](../../mechanics/effects/composed_test.go).

- Provider and S1 CI passed at their recorded source heads; the stacked S2 base still does not trigger automatic PR CI. No provider merge/tag or final released consumer pin exists yet — [remaining adoption](../../docs/ideas/item-effect-composition/implementation.md#still-required-before-final-adoption).

## Mistaken

- The initial plan treated a top-level effects home as a recommendation, not a ruling; the operator chose mechanics and its existing tag history was checked before promotion. Git staging initially named the removed old directory after git mv; the already-staged deletion and new destination were then verified explicitly — [plan](../../docs/ideas/item-effect-composition/plan.md).
