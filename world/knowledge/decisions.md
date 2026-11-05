# Decisions — world

One line per entry, dated and linked. Ingest is the writer; entries arrive through its reviewed PR.
A decision reads `- <date> · <the words> · options: <the options it closed> · because: <why> · <link> · upheld: <links, or none yet>`;
a finding, friction or open line reads `- <date> · <the words> · <link>`. A finding that needs more than a line links its case file in `findings/`.
Friction names both entries and never settles them.

## Decisions

- 2026-11-03 · newTracker reports no authored goals with unexported errNoGoals and New consumes only that signal, leaving the tracker absent · options: nil,nil; duplicate len(Goals) gating at the caller; exporting the internal signal · because: the builder owns what no goals means, and goals without a clock remain goal.NewTracker’s refusal · https://github.com/KirkDiggler/rpg-toolkit/blob/6d8a753a4e255e38040b1b63a2bfb731832095c2/world/knowledge/runs/2026-11-03-nilnil-2220-world-no-goals.md · upheld: none yet

## Findings

- none yet

## Friction

- none yet

## Open

- none yet
