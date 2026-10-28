# Decisions — the repo root

One line per entry, dated and linked. Ingest is the writer; entries arrive through its reviewed PR.
A decision reads `- <date> · <the words> · options: <the options it closed> · because: <why> · <link> · upheld: <links, or none yet>`;
a finding, friction or open line reads `- <date> · <the words> · <link>`. A finding that needs more than a line links its case file in `findings/`.
Friction names both entries and never settles them.

## Decisions

- 2026-10-27 · a primitive the encounter composes with (play/*, mind/*, tools/*, world) works for any rulebook and is never rulebook-built; the dnd5e encounter is rulebook-specific and may know its rulebook, so the protected seam is primitive ↔ rulebook, not encounter ↔ rulebook (the operator calls this "an actual law") · options: primitives built for one rulebook, primitives that serve any rulebook · because: play/* and world are the model to copy, one concern each and deliberately ignorant of everything else ([rpg-project CLAUDE.md, the lens](https://github.com/KirkDiggler/rpg-project/blob/main/CLAUDE.md#the-lens--read-this-before-anything-else)) · [rpg-project#694](https://github.com/KirkDiggler/rpg-project/issues/694) · upheld: none yet
- 2026-10-27 · an encounter may be built outside the rulebooks, for example a clock and intel management layer, and the dnd5e encounter may use it; the operator calls that "not only fine but a good way forward" · options: every encounter lives inside a rulebook, a generic encounter beneath the dnd5e one · because: a new complex thing first looks for the tool upgrade that makes it composable, pluggable and extensible · [rpg-project#694](https://github.com/KirkDiggler/rpg-project/issues/694) · upheld: none yet

## Findings

- none yet

## Friction

- none yet

## Open

- 2026-10-27 · direction, not decided: a generic encounter that manages clocks (play/clock) and intel hooks (play/intel) as a tool, with the basics built and the rulebook managing the resources that activate them, is the safety valve to test against when the dnd5e encounter's logic strains. Before proposing it, measure what in the dnd5e encounter is actually dnd5e-specific; its [go.mod](https://github.com/KirkDiggler/rpg-toolkit/blob/c355102dffe40fa6aa3852d13708ec21c055172d/rulebooks/dnd5e/encounter/go.mod) already requires no rulebooks/dnd5e · [rpg-project#694](https://github.com/KirkDiggler/rpg-project/issues/694)
