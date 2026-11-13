# Run — 2026-11-13 — Composable conditions foundation review

Design work for [rpg-project#703](https://github.com/KirkDiggler/rpg-project/issues/703). Base: toolkit main at [c55c9a06](https://github.com/KirkDiggler/rpg-toolkit/commit/c55c9a06).

## Shipped

- No runtime change or release; this contribution is a concrete authoring/selection proposal and acceptance surface — [proposal](../../docs/ideas/bless-bane-building-blocks/README.md).

## Decided

- KirkDiggler selected Bless/Bane as the first foundation cleanup and agreed that instance coexistence, applicability, oldest-per-group selection and signed additive combination are distinct; exact exported API and module placement remain proposed — [direction](https://github.com/KirkDiggler/rpg-project/issues/703#issuecomment-6863768422).

## Found

- Character save/load and monster carried-condition preparation preserve insertion order; the existing roll selector already takes the first applicable contributor per group, providing a seam to extract rather than a second selection engine to invent — [ordering proposal](../../docs/ideas/bless-bane-building-blocks/README.md#walk-one-roll-through).
- Fresh tests pass in effects, conditions, contributions, character, monster, monstertraits and rolls, plus both neutral effects packages; this is baseline evidence, not proof of the unimplemented proposal — [acceptance boundary](../../docs/ideas/bless-bane-building-blocks/acceptance.md#evidence-boundaries).

## Mistaken

- The first draft assigned total combination to rolls.ResolveContributions; inspection shows it returns signed physical components to the owning operation, and the diagram/table now preserve that boundary — [ownership](../../docs/ideas/bless-bane-building-blocks/README.md#ownership-and-contracts).
