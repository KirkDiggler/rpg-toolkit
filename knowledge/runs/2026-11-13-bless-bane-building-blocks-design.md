# Run — 2026-11-13 — Composable conditions foundation review

Design work for [rpg-project#703](https://github.com/KirkDiggler/rpg-project/issues/703). Base: toolkit main at [c55c9a06](https://github.com/KirkDiggler/rpg-toolkit/commit/c55c9a06).

## Shipped

- No runtime change or release; this contribution is a concrete authoring/selection proposal and acceptance surface — [proposal](../../docs/ideas/bless-bane-building-blocks/README.md).

## Decided

- KirkDiggler selected Bless/Bane as the first foundation cleanup and agreed that instance coexistence, applicability, oldest-per-group selection and signed additive combination are distinct — [direction](https://github.com/KirkDiggler/rpg-project/issues/703#issuecomment-6863768422).
- KirkDiggler approved the design direction with R4's concrete API allowed to evolve, and made ease of composing a concentration-maintained raging ring a success measure; the lead's plan shares native Rage benefit declarations without moving its lifecycle, and treats a generic overrides system as exploratory rather than required — [approval and scope](https://github.com/KirkDiggler/rpg-toolkit/pull/2425#issuecomment-6865141821).

- On 2026-11-14 KirkDiggler accepted the effect-first direction: one complete Raging definition with discoverable typed fields, general configurable effects/compositions, shared code/config values and symmetric upkeep reuse. The first-cut execution plan is retained but superseded for continuing work; concrete canonical/reference/native-maintenance contracts must be re-cut before API adoption — [direction change](https://github.com/KirkDiggler/rpg-toolkit/pull/2425#issuecomment-6869803445).

- The lead proposes R9 as an ordered named canonical tree lowered to flat runtime parts, with explicit placement rather than implicit group lifetimes; this is a proposal awaiting agreement, not an operator-approved runtime change — [R9 record](https://github.com/KirkDiggler/rpg-toolkit/pull/2425#issuecomment-6873310528).

## Found

- Character save/load and monster carried-condition preparation preserve insertion order; the existing roll selector already takes the first applicable contributor per group, providing a seam to extract rather than a second selection engine to invent — [ordering proof](../../docs/ideas/bless-bane-building-blocks/implementation.md#what-the-root-proof-establishes).
- Fresh tests pass in effects, conditions, contributions, character, monster, monstertraits and rolls, plus both neutral effects packages; this is baseline evidence, not proof of the unimplemented proposal — [acceptance boundary](../../docs/ideas/bless-bane-building-blocks/acceptance.md#evidence-boundaries).

- At runtime source c99f3d85 the public composition still requires coordinator/benefits/maintenance and a catalogue entry, while native Rage keeps a private round-anchored upkeep recipe; the generic Duration part counts holder ends, so a field wrapper or direct substitution cannot establish the new parity contract — [measured starting point](https://github.com/KirkDiggler/rpg-toolkit/pull/2425#issuecomment-6869803445).

- Existing PartDeclaration combines config with key/terminal scope, while Assembly owns ordered flat parts, namespace and end routing and refuses nested lifecycle hooks; pure definition compilation can be separated from placement without inventing another runtime — [inspected seam](https://github.com/KirkDiggler/rpg-toolkit/pull/2425#issuecomment-6873310528).

## Mistaken

- The first ring proof established benefit reuse but not symmetric reuse of a complete definition; the revised acceptance includes native Rage and a resistance-only ward borrowing its upkeep through both code and configuration — [revised acceptance](../../docs/ideas/bless-bane-building-blocks/acceptance.md).
- The first draft assigned total combination to rolls.ResolveContributions; inspection shows it returns signed physical components to the owning operation, and the diagram/table now preserve that boundary — [ownership](../../docs/ideas/bless-bane-building-blocks/README.md#ownership-and-contracts).
