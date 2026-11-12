# Run — 2026-11-03 — Composable conditions S8: retire legacy proficiency

processed: 2026-11-12T165555Z

Slice [rpg-project#853](https://github.com/KirkDiggler/rpg-project/issues/853) of [rpg-project#703](https://github.com/KirkDiggler/rpg-project/issues/703). Stacked on [features retirement #2261](https://github.com/KirkDiggler/rpg-toolkit/pull/2261).

## Shipped

- Prepared removal of `mechanics/proficiency` and its active map/navigation/quality entries; the rulebook's proficiency implementation remains unchanged — [retirement scope](https://github.com/KirkDiggler/rpg-project/issues/853).

## Decided

- Retire the second orphaned consumer before the legacy effects provider, without migrating unused code to the new prototype; preserve historical records — [operator direction and plan](https://github.com/KirkDiggler/rpg-project/issues/853).

## Found

- The source/module census finds no importer of `mechanics/proficiency` outside itself; after this removal no surviving source module imports legacy `mechanics/effects` — [consumer audit](https://github.com/KirkDiggler/rpg-project/issues/853).

## Mistaken

- None observed in this module removal — [bounded plan](https://github.com/KirkDiggler/rpg-project/issues/853).
