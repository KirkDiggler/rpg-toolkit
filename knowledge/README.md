# knowledge/ — the repo root

This directory holds the knowledge of the repo root: the rules that bind the modules directly beneath it. A rule that binds one of those modules lives in that module's own `knowledge/`; only a rule that crosses layers lives here.

[`map.md`](map.md) is the seam document and the diagram for this level. A person draws it, in https://github.com/KirkDiggler/rpg-toolkit/pull/2136; ingest never writes it.

[`decisions.md`](decisions.md) holds one line per decision, finding, friction and open question that binds this level. Ingest is its writer; a contributor does not edit it by hand.

`runs/` holds run records. A work PR whose changes bind this level commits one record there, with four sections — shipped, decided, found, mistaken — each line linked to its PR, issue or commit. A note in the workspace inbox is the other way to contribute. Ingest reads both. `runs/` and `findings/` (case files, present tense, evidence linked) are created by the first entry that needs them.

The rules these come from are [team memory, standing rules 13–15](https://github.com/KirkDiggler/rpg-project/blob/main/ideas/team-memory/design.md#standing-rules).
