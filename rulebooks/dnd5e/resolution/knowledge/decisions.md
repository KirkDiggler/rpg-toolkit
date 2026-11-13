# Decisions — rulebooks/dnd5e/resolution

One line per entry, dated and linked. Ingest is the writer; entries arrive through its reviewed PR.
A decision reads `- <date> · <the words> · options: <the options it closed> · because: <why> · <link> · upheld: <links, or none yet>`;
a finding, friction or open line reads `- <date> · <the words> · <link>`. A finding that needs more than a line links its case file in `findings/`.
Friction names both entries and never settles them.

## Decisions

- 2026-10-28 · resolution dispatches on a targeting-gate profile, never on Sanctuary's name: the condition declares its targeting gate (an attacker's save before any roll against the warded target) and its self-break clause as data, resolution runs the gate by kind, and refs.Spells.Sanctuary() and *conditions.SanctuaryCondition leave this module · options: recording the Sanctuary dispatch as an accepted exception until a second targeting-gate spell arrives · because: ADR-0045's reason is exactly what the dispatch breaks, a new spell must never need resolution code; the concentration work upheld it on 2026-09-08; the change landed before the docs that would have caught it existed (the operator, 2026-10-28) · [rpg-toolkit#2140](https://github.com/KirkDiggler/rpg-toolkit/issues/2140) · upheld: [rpg-project#407](https://github.com/KirkDiggler/rpg-project/pull/407) (2026-09-08: a condition carries why it ended; resolution looks nothing up by identity)

## Findings

- 2026-11-12 · A frozen cast retains execution position and pending delivery against the existing linked application, not a second lifetime. A native pause after some recipients land must survive save/reload without redelivering the earlier recipients or delaying their effects until every recipient finishes. · https://github.com/KirkDiggler/rpg-toolkit/blob/1f66f3c7dc1a1d293d19cda89a7a4f1b9397613d/rulebooks/dnd5e/resolution/knowledge/runs/2026-11-09-resumable-initial-delivery.md
- 2026-11-12 · deliverMove’s always-nil immediate error is constrained by the continuation shape: its returned Gather can fail later. The local unparam exception documents that deferred error rather than fabricating an immediate failure path. · https://github.com/KirkDiggler/rpg-toolkit/blob/1f66f3c7dc1a1d293d19cda89a7a4f1b9397613d/rulebooks/dnd5e/resolution/knowledge/runs/2026-11-04-2254-resolution-drain.md

## Friction

- none yet

## Open

- none yet
