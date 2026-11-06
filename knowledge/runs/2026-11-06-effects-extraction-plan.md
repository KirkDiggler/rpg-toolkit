# Run — 2026-11-06 — Items-first effects extraction planning

Slice [rpg-project#705](https://github.com/KirkDiggler/rpg-project/issues/705) under [story #703](https://github.com/KirkDiggler/rpg-project/issues/703), consuming [item S1](https://github.com/KirkDiggler/rpg-project/issues/862) and [Rage/loot S2](https://github.com/KirkDiggler/rpg-project/issues/873).

## Shipped

- No runtime extraction or release; a concrete first-cut plan links the accepted decision and walkthrough to provider/consumer files, contracts and checks, with H1 explicitly blocking execution — [plan](../../docs/ideas/item-effect-composition/plan.md).

## Decided

- The operator cleared toolkit#2288 and project#878 for merge; they now provide the canonical walkthrough and record/skill map that this plan derives from — [documentation work #876](https://github.com/KirkDiggler/rpg-project/issues/876).
- Recommend a top-level effects module with machine inside the same release unit; do not silently settle the module-home question the earlier design deliberately left until extraction — [H1 and alternatives](../../docs/ideas/item-effect-composition/plan.md#scope-and-unresolved-choice).
- Keep this cut mechanical and items-first; rulebook vocabulary, condition factories/migration and editor descriptors remain separate contracts rather than hidden prerequisites — [scope](../../docs/ideas/item-effect-composition/plan.md#scope-and-unresolved-choice).

## Found

- The live Go-import census places provider imports in examples/effects and consumer imports in examples/items; historical origin notes and pinned run records also name the example path but are not dependencies — [inspected baseline and migration tasks](../../docs/ideas/item-effect-composition/plan.md).
- CI discovers surviving module directories and skips deleted paths with no remaining go.mod; the tagger excludes examples and initializes a new ordinary module, so a move and its real first tag need distinct evidence — [release task](../../docs/ideas/item-effect-composition/plan.md#task-3--releaseadoption-evidence-only-with-the-required-authority).

## Mistaken

- No source move or consumer re-pin is claimed; planning completeness is not execution readiness while H1 remains open — [plan status](../../docs/ideas/item-effect-composition/plan.md).
