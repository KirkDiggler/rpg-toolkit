# ADR-0035: Persist one canvas floor mask and its envelope

**Status:** Accepted
**Date:** 2026-08-09
**Issue:** rpg-toolkit#897

## Context

Dungeon YAML v0.3 treated every in-bounds canvas cell as floor. v0.4 adds
`canvas.floor_source: regions`, where bounds remain workspace legality but only
the canonical union of `regions[].cells` is playable. Re-deriving either a
rectangle or union independently in placement, movement, LoS, fog, or reload
would create multiple authorities and let in-bounds void leak into mechanics.

Authored documents and running encounters also have different lifecycles:
a complete authored candidate is recompiled standalone, while a running
encounter must survive later source edits unchanged.

## Decision

Canvas compilation produces one canonical sorted floor mask and one canonical
set of normalized floor/void envelope pairs. `DungeonParams` carries both into
`InitDungeon`; `SpaceData.FloorCells` and `SpaceData.EnvelopeEdges` persist the
exact started snapshot. Reload validates canonical order, envelope completeness,
edge ownership, every entity/obstacle/start position, and remembered fog cells
against that snapshot. It never consults authored YAML.

A masked runtime grid makes only persisted floor cells valid. Movement/pathing,
placement and spatial queries consume that grid. The room wrapper rejects LoS
rays that cross a void cell, and perception rejects invalid targets before
adding them to visible/remembered knowledge. Envelope records project from the
single floor owner only; void cells never gain runtime records.

`dungeonspec.CompileDungeon` is the complete-candidate provider seam. Draft mode
projects structurally valid empty/tiny/disconnected masks with an optional
entrance. Strict mode additionally requires a nonempty connected floor and a
complete same-component party-start reservation. Authored validation is returned
as ordered `FieldError` records; cancellation/provider failures remain Go errors.
No previous compiled candidate is accepted.

Legacy canvas snapshots that omit the new fields continue to reload as the v0.3
bounds rectangle. Newly initialized bounds canvases persist their full rectangle
and outer envelope explicitly.

## Consequences

- Region edits cannot inherit stale occupancy, entrance, or party seats.
- In-bounds void is mechanically identical to off-canvas space for placement,
  pathing, movement, reachability, LoS and fog.
- Running encounters retain the exact mask/edges they started with.
- The envelope wire stays the existing undirected pair representation, including
  actual off-canvas neighbor coordinates; no void `HexRecord` is introduced.
- Room-chain generation and its existing region/connector behavior are unchanged.
