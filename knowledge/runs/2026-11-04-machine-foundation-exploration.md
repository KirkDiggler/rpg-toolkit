# Run — 2026-11-04 — NPC machine foundation exploration

Story [rpg-project#859](https://github.com/KirkDiggler/rpg-project/issues/859). Base: toolkit main at [`532f4621`](https://github.com/KirkDiggler/rpg-toolkit/commit/532f46217ef01e51a5f746a87ab14d94a0d22a7e). This is a documentation exploration, not an implementing slice.

## Shipped

- Designer-facing exploration: component shape, existing type boundaries, proposed connections, gate-to-ballroom walkthrough, contrasting mind/effects cases and unresolved seams; no runtime changes — [walkthrough](../../docs/ideas/machine-foundation/README.md).

## Decided

- KirkDiggler, 2026-11-04: explore the primitive inside `mind/machine` before extraction, keep effects separate, and use the keep's journeys to make the boundaries concrete; no shared API or implementation plan is approved by that direction — [story](https://github.com/KirkDiggler/rpg-project/issues/859).
- Treat the keep examples as design probes rather than new implementation slices or playtest launch gates; preserve the existing admission/arrest priority and separate vocabulary review — [scope and evidence](https://github.com/KirkDiggler/rpg-project/issues/859#issuecomment-6642383444).

## Found

- The encounter's machine input delivery waits for a world-clock raise and excludes fight-clock actors, while plain speech pays no round and effects handles bus inputs synchronously; information delivery and action permission need separate consideration — [source-pinned evidence](https://github.com/KirkDiggler/rpg-project/issues/859#issuecomment-6642383444).
- `Option.Requires` reads the speaker's knowledge; this is not a target-state or target-belief eligibility contract — [source-pinned evidence](https://github.com/KirkDiggler/rpg-project/issues/859#issuecomment-6642383444).
- Existing state-bound posts keep spatial meaning outside `machine.Recipe`, while the encounter wake event does not expose sound kind and its sighting wake is limited to opposed subjects — [source-pinned evidence](https://github.com/KirkDiggler/rpg-project/issues/859#issuecomment-6642383444).
- Three unchanged-code package baselines pass across mind and effects; all 23 relative walkthrough links resolve, with 28 modules mapped and 49 ADRs indexed; this is not proof of implemented keep journeys — [verification](https://github.com/KirkDiggler/rpg-project/issues/859#issuecomment-6642383444).

## Mistaken

- None observed; the explored connections are explicitly proposals rather than a claim that the current generic step already supplies richer encounter inputs or consequences — [scope](https://github.com/KirkDiggler/rpg-project/issues/859).
