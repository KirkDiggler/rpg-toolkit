# Run — 2026-11-04 — NPC machine foundation exploration

processed: 2026-11-12T165555Z

Story [rpg-project#859](https://github.com/KirkDiggler/rpg-project/issues/859). Base: toolkit main at [`532f4621`](https://github.com/KirkDiggler/rpg-toolkit/commit/532f46217ef01e51a5f746a87ab14d94a0d22a7e). This is a documentation exploration, not an implementing slice.

## Shipped

- Designer-facing exploration: component shape, existing type boundaries, proposed connections, gate-to-ballroom walkthrough, contrasting mind/effects cases and unresolved seams; no runtime changes — [walkthrough](../../docs/ideas/machine-foundation/README.md).

- 2026-11-05: Proposed ADR-0049 records `mind/behavior` as a composer, separating reusable definitions/private instance progress from generation's parameter-selection policy; indexed as not implemented, with no other ADR status changed — [proposal](../../docs/adr/0049-behavior-is-a-composer.md).

## Decided

- KirkDiggler, 2026-11-04: explore the primitive inside `mind/machine` before extraction, keep effects separate, and use the keep's journeys to make the boundaries concrete; no shared API or implementation plan is approved by that direction — [story](https://github.com/KirkDiggler/rpg-project/issues/859).
- Treat the keep examples as design probes rather than new implementation slices or playtest launch gates; preserve the existing admission/arrest priority and separate vocabulary review — [scope and evidence](https://github.com/KirkDiggler/rpg-project/issues/859#issuecomment-6642383444).

- KirkDiggler, 2026-11-05: procedural generation and suitable parameter values are part of the authoring goal, and the behavior-composer direction deserves an ADR proposal; this does not accept its public APIs, generation profiles or implementation — [direction and evidence](https://github.com/KirkDiggler/rpg-project/issues/859#issuecomment-6648962999).

- 2026-11-05: KirkDiggler approved proceeding with the composer responsibility and mechanical parameter metadata; a separate quality-profile framework is not required up front, and implementation proceeds through S1 without merge authorization — [agreement](https://github.com/KirkDiggler/rpg-project/issues/866#issuecomment-6650897115).

## Found

- The encounter's machine input delivery waits for a world-clock raise and excludes fight-clock actors, while plain speech pays no round and effects handles bus inputs synchronously; information delivery and action permission need separate consideration — [source-pinned evidence](https://github.com/KirkDiggler/rpg-project/issues/859#issuecomment-6642383444).
- `Option.Requires` reads the speaker's knowledge; this is not a target-state or target-belief eligibility contract — [source-pinned evidence](https://github.com/KirkDiggler/rpg-project/issues/859#issuecomment-6642383444).
- Existing state-bound posts keep spatial meaning outside `machine.Recipe`, while the encounter wake event does not expose sound kind and its sighting wake is limited to opposed subjects — [source-pinned evidence](https://github.com/KirkDiggler/rpg-project/issues/859#issuecomment-6642383444).
- Three unchanged-code package baselines pass across mind and effects; all 23 relative walkthrough links resolve, with 28 modules mapped and 49 ADRs indexed; this is not proof of implemented keep journeys — [verification](https://github.com/KirkDiggler/rpg-project/issues/859#issuecomment-6642383444).

- 2026-11-05: existing keepgen emits validated garrison/table/state bindings; the three focused generation tests pass (1.362s), but do not establish an editable behavior library or good parameter profiles — [receipt](https://github.com/KirkDiggler/rpg-project/issues/859#issuecomment-6648962999).
- 2026-11-05: 0048 is claimed by both the equipment proposal and an older encounter-backfill issue; the conflict is reported to the existing owner, and this proposal uses 0049 without changing another session's work — [numbering notice](https://github.com/KirkDiggler/rpg-toolkit/issues/2142#issuecomment-6648840808).

## Mistaken

- 2026-11-04: none observed; the explored connections are explicitly proposals rather than a claim that the current generic step already supplies richer encounter inputs or consequences — [scope](https://github.com/KirkDiggler/rpg-project/issues/859).
- 2026-11-05: the initial ADR-0049 borrowed standing authority from older ADRs because names/principles looked related; operator review identified different architectural eras, so the proposal now cites current play/world/layering and inspected execution/knowledge contracts, with older ADRs explicitly historical and unchanged — [visible correction](../../docs/adr/0049-behavior-is-a-composer.md#historical-context-not-inherited-authority).
