# Run — 2026-11-03 — Composable conditions S8: retire legacy effects

processed: 2026-11-05T201556Z

Slice [rpg-project#853](https://github.com/KirkDiggler/rpg-project/issues/853) of [rpg-project#703](https://github.com/KirkDiggler/rpg-project/issues/703). Stacked on [proficiency retirement #2262](https://github.com/KirkDiggler/rpg-toolkit/pull/2262), after [features retirement #2261](https://github.com/KirkDiggler/rpg-toolkit/pull/2261).

## Shipped

- Prepared removal of legacy `mechanics/effects` and its active module/map/health guidance, including current docs that still proposed migrating already-retired mechanics modules — [retirement plan](https://github.com/KirkDiggler/rpg-project/issues/853).
- The stack leaves 28 mapped modules and no surviving Go source or go.mod reference to the three retired modules; race tests pass for resources, combat, features and character, and both independent machine suites pass — [verification record](https://github.com/KirkDiggler/rpg-project/issues/853).

## Decided

- Keep live `mechanics/resources`, both current state-machine implementations and all rulebook code/pins byte-identical to the base; retirement is not prototype promotion or an abstraction project — [operator direction and scope](https://github.com/KirkDiggler/rpg-project/issues/853).
- Rewrite the current mechanics guide around its remaining resources consumer and preserve the old reasoning in ADRs/journey/run records rather than advertise retired APIs as usable — [retirement scope](https://github.com/KirkDiggler/rpg-project/issues/853).

## Found

- Several current component/status/quality pages still called `mechanics/conditions` and `mechanics/spells` active and proposed an events migration, although their deletion predates this stack; those present-tense claims are removed, not their historical records — [earlier retirement issue](https://github.com/KirkDiggler/rpg-toolkit/issues/973).
- Deletion-aware CI has no surviving changed module to test, so the live-consumer census and explicit local consumer suites are the meaningful proof, alongside the map check — [verification plan](https://github.com/KirkDiggler/rpg-project/issues/853).

## Mistaken

- The old promotion plan described effects as having no importers, omitting its two orphaned consumers; this stack removes consumers first rather than keeping them or assuming they never existed — [corrected finding](https://github.com/KirkDiggler/rpg-project/issues/705#issuecomment-6619076748).
