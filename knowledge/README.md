# knowledge/ — the repo root

This directory holds the knowledge of the repo root: the rules that bind the
layers directly beneath it — Core, Mechanics, Tools, Play, Mind, World,
Content, Rulebooks and the example spikes. A rule that binds one module lives
in that module; a rule that binds the modules of one level lives in that
level's `knowledge/`. Only a rule that crosses layers lives here.

[`map.md`](map.md) is the index. It draws the layers, names what crosses each
seam between them and what must not, and composes the level maps: a level with
its own `knowledge/map.md` appears here as one box that links to it, and its
modules are drawn only there.

The level maps are [`rulebooks/dnd5e/knowledge/map.md`](../rulebooks/dnd5e/knowledge/map.md)
(encounter, resolution, session and the rules module) and
[`play/knowledge/map.md`](../play/knowledge/map.md) (the four play leaves).

[`decisions.md`](decisions.md) holds one line per ruling that binds this level.
Ingest is its writer; a contributor does not edit it by hand.

[`runs/`](runs/) holds run records. A work PR whose changes bind this level
commits one record here, with four sections — shipped, decided, found,
mistaken — each line linked to its PR, issue or commit. That record is how a
contributor without the workspace tooling adds to this directory; a note in
the workspace inbox is the other way. Ingest reads both.

`findings/` holds case files: present tense, evidence linked. It is created by
the first finding that needs it.

`scripts/check-knowledge-map.sh` runs in CI and fails when a module with a
`go.mod` appears in no map, in more than one, or in a map other than the
nearest one above it, or when a map names a module that has no `go.mod`. A PR
that adds, moves or retires a module edits the map in the same PR.

The design these rules come from is
[team memory, standing rules 13–15](https://github.com/KirkDiggler/rpg-project/blob/main/ideas/team-memory/design.md#standing-rules).
