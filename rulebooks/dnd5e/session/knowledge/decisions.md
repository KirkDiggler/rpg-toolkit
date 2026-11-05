# Decisions — rulebooks/dnd5e/session

One line per entry, dated and linked. Ingest is the writer; entries arrive through its reviewed PR.
A decision reads `- <date> · <the words> · options: <the options it closed> · because: <why> · <link> · upheld: <links, or none yet>`;
a finding, friction or open line reads `- <date> · <the words> · <link>`. A finding that needs more than a line links its case file in `findings/`.
Friction names both entries and never settles them.

## Decisions

- none yet

## Findings

- 2026-11-03 · Landing Record and Window closures consume their own errNoUnit/errNoWindow signals before returning to the verb runner; propagating those non-events would refuse successful verbs. The workbench DiscoverySharing fake uses ErrNotFound for an unchosen preference. · https://github.com/KirkDiggler/rpg-toolkit/blob/6d8a753a4e255e38040b1b63a2bfb731832095c2/rulebooks/dnd5e/session/knowledge/runs/2026-11-03-nilnil-2220-session-drain.md
- 2026-10-30 · An ended beat’s outcome folds into ClockEntry.Outcome; an absent result remains empty, and the closed run can be read through a fresh manager. The result is not duplicated onto session.Outcome. · https://github.com/KirkDiggler/rpg-toolkit/blob/6d8a753a4e255e38040b1b63a2bfb731832095c2/rulebooks/dnd5e/knowledge/runs/2026-10-30-franks-list-s9-screen.md
- 2026-11-04 · Launch carries BlockingStates and Posts beside the compiled recipe, and View copies ObservedHolds rather than recomputing alliance/pass policy. Move translates body/held-crossing refusals into stopped plus the public reason and charges only walked cells; Atlas and reveal conversions preserve footprint crossings as marked doorways. · https://github.com/KirkDiggler/rpg-toolkit/blob/6d8a753a4e255e38040b1b63a2bfb731832095c2/rulebooks/dnd5e/knowledge/runs/2026-11-03-the-guard-holds-the-gate-s1-session.md, https://github.com/KirkDiggler/rpg-toolkit/blob/6d8a753a4e255e38040b1b63a2bfb731832095c2/rulebooks/dnd5e/knowledge/runs/2026-11-04-the-guard-holds-the-gate-s1b-session.md, https://github.com/KirkDiggler/rpg-toolkit/blob/6d8a753a4e255e38040b1b63a2bfb731832095c2/rulebooks/dnd5e/knowledge/runs/2026-11-04-the-guard-holds-the-gate-s1c-session.md
- 2026-11-04 · Unlock checks DoorHeldFor before staging any lock roll or Guidance window. If a door becomes held during a paused unlock, the resume closes the window declined, commits, and then returns the typed held refusal; dropping that scope would leave the window live and the session frozen. · https://github.com/KirkDiggler/rpg-toolkit/blob/6d8a753a4e255e38040b1b63a2bfb731832095c2/rulebooks/dnd5e/knowledge/runs/2026-11-04-the-guard-holds-the-gate-s1b-session.md

## Friction

- none yet

## Open

- none yet
