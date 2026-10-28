# knowledge/ — play

This directory holds the knowledge of the play level: the rules that bind the
four leaves directly beneath it — `clock`, `intel`, `interrupt` and `record`.
A rule about one leaf lives in that leaf's design contract,
`docs/ideas/play/<name>/design.md`; a rule about how the leaves meet a
composition lives in the [root knowledge](../../knowledge/).

[`map.md`](map.md) is the seam document and the diagram for this level: what
each leaf may import, and what must not cross between any two of them.

[`decisions.md`](decisions.md) holds one line per ruling that binds this
level. Ingest is its writer; a contributor does not edit it by hand.

`runs/` holds run records. A work PR whose changes bind this level commits one
record there, with four sections — shipped, decided, found, mistaken — each
line linked to its PR, issue or commit. A note in the workspace inbox is the
other way to contribute. Ingest reads both. `runs/` and `findings/` (case
files, present tense, evidence linked) are created by the first entry that
needs them.

A leaf added beneath this level, or one retired, is drawn in or out of
`map.md` in the same PR; `scripts/check-knowledge-map.sh` fails CI otherwise.
