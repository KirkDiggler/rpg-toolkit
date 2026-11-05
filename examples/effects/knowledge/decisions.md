# Decisions — examples/effects

One line per entry, dated and linked. Ingest is the writer; entries arrive through its reviewed PR.
A decision reads `- <date> · <the words> · options: <the options it closed> · because: <why> · <link> · upheld: <links, or none yet>`;
a finding, friction or open line reads `- <date> · <the words> · <link>`. A finding that needs more than a line links its case file in `findings/`.
Friction names both entries and never settles them.

## Decisions

- none yet

## Findings

- 2026-11-04 · The effects prototype constructs through effects.New(effects.Config) and machine.New(machine.Config); the composed constructor forwards saved State, the machine uses a supplied bag without copying and clones defaults for nil Bag, and Apply retains unknown/terminal-state validation. This is ergonomic cleanup, not promotion or lifecycle redesign. · https://github.com/KirkDiggler/rpg-toolkit/blob/6d8a753a4e255e38040b1b63a2bfb731832095c2/knowledge/runs/2026-10-28-composable-conditions-s1.md, https://github.com/KirkDiggler/rpg-toolkit/blob/6d8a753a4e255e38040b1b63a2bfb731832095c2/examples/effects/knowledge/runs/2026-11-04-explicit-construction-inputs.md
- 2026-10-28 · Blob flattens its declared bag with json/v2’s embed tag and refuses header-key collisions; its explicit state header means old/new condition blobs cannot be asserted byte-equal without accounting for that field. Rage-shaped anchoring sets the first available round only and derives turns_active on the graced turn too. · https://github.com/KirkDiggler/rpg-toolkit/blob/6d8a753a4e255e38040b1b63a2bfb731832095c2/knowledge/runs/2026-10-28-composable-conditions-s1.md

## Friction

- none yet

## Open

- none yet
