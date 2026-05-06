package perception

import (
	"github.com/KirkDiggler/rpg-toolkit/encounter/events"
	"github.com/KirkDiggler/rpg-toolkit/encounter/types"
)

// ProjectMove computes a viewer's move slice and reveal slice when an entity
// moves along path. Returns (moveSlice, revealSlice). Either may be nil if
// the viewer perceives nothing of the move or has no vision change.
//
// Slice 1 stub: viewer's visibility is computed from their CURRENT position.
// Real LoS will be position-aware-per-segment. Slice 1 does NOT emit
// EntityVisibility — entity-knowledge accumulation is a future slice.
//
// The mover parameter is reserved for future slices (entity-visibility
// accumulation will use it to record "X became visible to viewer").
func ProjectMove(
	_ types.EntityID, // mover — reserved for future-slice entity-visibility
	path []types.Hex,
	viewer *PerceptionView,
) (moveSlice *events.MovePlayerSlice, revealSlice *events.HexRevealedSlice) {
	if viewer == nil || len(path) == 0 {
		return nil, nil
	}
	visible := VisibleHexesAt(viewer.Position, viewer.SightRange)

	var seen []types.Hex
	for _, hex := range path {
		if visible.Has(hex) {
			seen = append(seen, hex)
		}
	}
	if len(seen) > 0 {
		moveSlice = &events.MovePlayerSlice{SeenSegments: seen}
	}
	// For someone-else's move from this viewer's perspective, the viewer's
	// own position didn't change, so under the stub LoS no new hexes are
	// revealed. Future slices handle entity-visibility deltas (mover entering
	// the viewer's vision becomes a EntityVisibility entry on the slice).
	return moveSlice, nil
}

// ProjectDoorOpen computes per-viewer slices when a door opens.
//
// Stub LoS: opening a door doesn't change which hexes the viewer can see
// (no walls modeled), but if the door is in the viewer's sight range we
// emit a DoorOpenedPlayerSlice. The reveal slice covers the door's
// immediate neighbors that the viewer hadn't seen before.
//
// The door and openedBy parameters are reserved for future slices (real LoS
// will need door identity for wall logic; entity-visibility accumulation
// for openedBy).
func ProjectDoorOpen(
	_ types.EntityID, // door — reserved for future-slice wall logic
	doorPos types.Hex,
	_ types.EntityID, // openedBy — reserved for future-slice entity-visibility
	viewer *PerceptionView,
) (doorSlice *events.DoorOpenedPlayerSlice, revealSlice *events.HexRevealedSlice) {
	if viewer == nil {
		return nil, nil
	}
	visible := VisibleHexesAt(viewer.Position, viewer.SightRange)
	if !visible.Has(doorPos) {
		return nil, nil
	}

	doorSlice = &events.DoorOpenedPlayerSlice{Visible: true}

	newHexes := make(types.HexSet)
	for _, neighbor := range HexNeighbors(doorPos) {
		if visible.Has(neighbor) && !viewer.RevealedHexes.Has(neighbor) {
			newHexes[neighbor] = struct{}{}
		}
	}
	if len(newHexes) > 0 {
		revealSlice = &events.HexRevealedSlice{Hexes: newHexes}
	}
	return doorSlice, revealSlice
}
