# knowledge/ — rulebooks/dnd5e

This directory holds the knowledge of the D&D 5e rulebook level: the rules
that bind exactly the modules directly beneath it — `rulebooks/dnd5e` (the
rules and content), `encounter`, `resolution` and `session`. A rule about one
of those modules lives in that module's own docs; a rule that crosses the
layers beneath the rulebook lives in the [root knowledge](../../../knowledge/).

[`map.md`](map.md) is the seam document and the diagram for this level. For
each seam it says what flows and what must not cross, in both directions. It
cites each module's `AGENTS.md` and the ADRs rather than restating them.

[`decisions.md`](decisions.md) holds one line per ruling that binds this
level. Ingest is its writer; a contributor does not edit it by hand.

`runs/` holds run records. A work PR whose changes bind this level commits one
record there, with four sections — shipped, decided, found, mistaken — each
line linked to its PR, issue or commit. A note in the workspace inbox is the
other way to contribute. Ingest reads both. `runs/` and `findings/` (case
files, present tense, evidence linked) are created by the first entry that
needs them.

A module added beneath this level, or one retired, is drawn in or out of
`map.md` in the same PR; `scripts/check-knowledge-map.sh` fails CI otherwise.
