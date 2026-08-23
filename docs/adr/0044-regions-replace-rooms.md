# ADR-0044: Regions Replace Rooms — a Region Is a Named Set of Absolute Cells

**Date:** 2026-08-23
**Status:** Accepted (ruled on rpg-project#255/#256, 2026-08-23; implemented in the PR that adds this file)

## Context

The session-stack encounter (`rulebooks/dnd5e/encounter`) authored a dungeon
as a CHAIN of rectangular chambers: `RoomInput{Width, Height, Origin}`,
`ConnectionInput` naming the one edge the seam wall left open, and a
compiler (`dungeonspec` version 1) that laid chambers left to right and
GENERATED the seam walls and doorway row between them. Three slices of
rework had already hollowed the room out: #1106 compiled the rooms onto one
canvas so a wall could be drawn across a seam at all; #1108 made a room a
"region" at runtime — a named set of cells, membership derived from a
member's cell — while keeping it a rectangle at authoring; #1127 made the
rectangle an OFFSET rectangle so hex fields constructed, at the cost of a
mask function, interval runs for overlap, a reverse conversion for
ownership, and a room-local → absolute seam (#1139) that was wrong twice
(#1141, #1150).

The dungeon builder (rpg-project#169, restarted as #256) needs none of that
and cannot use it: a builder paints cells, and a rectangle-plus-anchor
cannot say an L-shaped hall, a cavern, or a wall anywhere but a seam.
Version 1's `archetype: entrance` also chose where the party stood — a word
about what a room is FOR silently deciding geometry, the rpg-toolkit#1033
trap.

## Decision

**A region is a named set of absolute cells carrying per-area world facts
(lighting now); rooms, origins and connections were the chain's vocabulary
and are gone.**

- `RegionInput{ID, Name, Cells []Position, Archetype, Lighting *Lighting}`
  replaces `RoomInput` + `Origin` + `ConnectionInput`. Cells are authored
  offset `[col,row]` under `CanvasInput.Orientation` (now REQUIRED) and
  converted ONCE, at construction, through `HexCellAt` — the one conversion
  in the package, pinned by `TestTheOneConversionIsCalledInOnePlace`. The
  floor is the union of the regions' cells; a cell in two regions is refused
  (`ErrRegionOverlap`), a region with none is refused (`ErrRegionEmpty`).
- `FieldInput{Canvas, Regions, Props, Walls, Doors}`: props and walls are
  FIELD facts at absolute authored cells. A wall is an edge between two
  adjacent floor cells; adjacency is spatial's answer under the declared
  orientation, never a parity table (`ErrEdgeNotAdjacent`,
  `ErrEdgeOffFloor`). The envelope is implied, never written.
- **Hex only.** The square family left with the chain. A second family is
  a second frame for every coordinate to be wrong in.
- `Archetype` is a presentation ref the assets resolve; it is REQUIRED,
  carried unread, and **never decides a mechanic**
  (`TestAnArchetypeNeverDecidesAMechanic`). `Lighting{Intensity ∈ [0,1]}`
  is REQUIRED per region (capabilities-supplied-never-defaulted, #1033) and
  carried unread; how an intensity becomes obscurement is the rulebook's.
- `Atlas` is flat: `Cells`, `Regions` (each with its cells, archetype,
  lighting), `Props`, `Boundaries`, `Doorways` (one per door edge, keyed by
  door ID), every list sorted. The session seam copies it through rather
  than re-enumerating rectangles.
- `StepOutput.Crossing` and the moved beat's `connection` go with the
  connection list; a step names the DOORS it went through. A doorway is the
  door standing in it — an open door where version 1 had an open connector.
- Persistence: `FieldData{canvas, regions, props, walls}`; `rooms` and
  `connections` are tombstones refused by name (the fail-loud rule,
  2026-08-17); `EndingData.At` replaces `Room` + `Position`.
- `dungeonspec` **version 2** is the file: `regions` / `start` / `walls` /
  `doors` / `place`, all absolute; `Validate` reports EVERY defect as a
  path-addressed `FieldError` so the builder can draw each on the canvas;
  `Compile` generates nothing. Version 1 is deleted and refused by name.

## Consequences

- The reference tomb is re-authored in version 2 and compiles to the
  IDENTICAL atlas version 1 produced — 224 cells, 28 walls, 2 doorways, 15
  props — pinned by a golden captured from v1 before v1 was deleted. That
  is the forcing case and the proof nothing moved under the combat fixtures.
- Every `encounter` fixture that built square rooms moved to hex regions.
  Two things followed that are facts about the model, not the port: a hex
  cell has six neighbours, so a seam wall built from three candidate pairs
  per row now emits only the pairs that touch; and a sightline hugging the
  edge column of a sheared rectangle passes through void cells, so under an
  opaque void two members on column 0 cannot see each other. Scenes about
  clocks or standing rather than about the void now declare a transparent
  one.
- `Atlas` is O(cells) again, deliberately: a region IS its cells, the
  budget (`maxFieldCells`) bounds what a field may list, and the report the
  host wants is the list the composition already holds. #1108's
  "describe, don't enumerate" was a property of rectangles.
- `rulebooks/dnd5e/session` (T3) copies regions through; rpg-api's
  `sessionworld` embed becomes `content/reference-tomb.yaml` in version 2.

## Rule

*A region is what it lists. Nothing about the floor, a wall, or a door is
derived from a shape, an anchor, or a word; the one conversion from the
authored frame happens once, at construction, and an archetype never decides
a mechanic.*
