# Decisions — rulebooks/dnd5e/resolution

One line per entry, dated and linked. Ingest is the writer; entries arrive through its reviewed PR.
A decision reads `- <date> · <the words> · options: <the options it closed> · because: <why> · <link> · upheld: <links, or none yet>`;
a finding, friction or open line reads `- <date> · <the words> · <link>`. A finding that needs more than a line links its case file in `findings/`.
Friction names both entries and never settles them.

## Decisions

- 2026-10-28 · resolution dispatches on a targeting-gate profile, never on Sanctuary's name: the condition declares its targeting gate (an attacker's save before any roll against the warded target) and its self-break clause as data, resolution runs the gate by kind, and refs.Spells.Sanctuary() and *conditions.SanctuaryCondition leave this module · options: recording the Sanctuary dispatch as an accepted exception until a second targeting-gate spell arrives · because: ADR-0045's reason is exactly what the dispatch breaks, a new spell must never need resolution code; the concentration work upheld it on 2026-09-08; the change landed before the docs that would have caught it existed (the operator, 2026-10-28) · [rpg-toolkit#2140](https://github.com/KirkDiggler/rpg-toolkit/issues/2140) · upheld: [rpg-project#407](https://github.com/KirkDiggler/rpg-project/pull/407) (2026-09-08: a condition carries why it ended; resolution looks nothing up by identity)

## Findings

- none yet

## Friction

- none yet

## Open

- none yet
