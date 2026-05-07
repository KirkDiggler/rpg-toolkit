package perception

import (
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/events"
)

// ProjectVisibilityTransition determines whether the mover entered or left the
// viewer's line of sight during this move, given the mover's starting position,
// their path (list of destination hexes), the move slice already projected by
// ProjectMove, and the viewer's precomputed visible hex set (from ProjectMove's
// same VisibleHexesAt call, passed here to avoid recomputation).
//
// Design: endpoints + SeenSegments boundaries (the simple "slice-1" approach).
// For each viewer, only the move endpoints and the seen-segment boundaries are
// checked — no per-hex timeline along the path. This is correct under the
// Manhattan stub LoS (monotonic visibility from a fixed viewer). Future real-LoS
// work can revisit; that is a follow-up issue.
//
//   - visibleAtStart = moverStart is in viewer's current LoS
//   - visibleAtEnd   = path[len-1] is in viewer's current LoS (the destination)
//   - (false, true)  → appearedAt = path[len-1]
//   - (true, false)  → disappearedAt = last hex of seenSegments, or moverStart
//     if seenSegments is empty (mover was visible at start but left immediately)
//   - (false, false) && seenSegments non-empty → pass-through: both
//     appearedAt = seenSegments[0], disappearedAt = seenSegments[len-1]
//   - (true, true)   → no visibility transition
//
// moverStart is the mover's position before the move (not in path; path contains
// only the destination hexes the mover traverses).
//
// visible is the precomputed result of VisibleHexesAt(viewer.Position,
// viewer.SightRange) — the same set used by ProjectMove for this viewer/path
// combination. Passing it in avoids a redundant O(range²) set construction per
// viewer per move.
//
// Returns (appearedAt, disappearedAt) — either may be nil for "no transition of
// that kind."
func ProjectVisibilityTransition(
	moverStart core.Hex,
	path []core.Hex,
	seenSegments []core.Hex,
	viewer *View,
	visible core.HexSet,
) (appearedAt, disappearedAt *core.Hex) {
	if viewer == nil || len(path) == 0 {
		return nil, nil
	}

	visibleAtStart := visible.Has(moverStart)
	visibleAtEnd := visible.Has(path[len(path)-1])

	switch {
	case !visibleAtStart && visibleAtEnd:
		// Mover entered LoS: appeared at path end (destination).
		h := path[len(path)-1]
		return &h, nil

	case visibleAtStart && !visibleAtEnd:
		// Mover left LoS: disappeared at last seen hex. When seenSegments is
		// empty the mover's start position itself was visible but no destination
		// hex in the path was, so moverStart is the last-known hex.
		if len(seenSegments) == 0 {
			return nil, &moverStart
		}
		h := seenSegments[len(seenSegments)-1]
		return nil, &h

	case !visibleAtStart && !visibleAtEnd && len(seenSegments) > 0:
		// Pass-through: mover traversed the viewer's LoS but is outside at both
		// endpoints. Both events fire.
		first := seenSegments[0]
		last := seenSegments[len(seenSegments)-1]
		return &first, &last

	default:
		// (true, true) or (false, false) with empty seen: no transition.
		return nil, nil
	}
}

// ProjectMove computes a viewer's move slice, reveal slice, and visible hex set
// when an entity moves along path. Returns (moveSlice, revealSlice, visible).
// moveSlice and revealSlice may be nil if the viewer perceives nothing of the
// move or has no vision change. visible is always non-nil.
//
// The visible set is returned so callers can pass it directly to
// ProjectVisibilityTransition without recomputing VisibleHexesAt (O(range²))
// a second time for the same viewer.
//
// Slice 1 stub: viewer's visibility is computed from their CURRENT position.
// Real LoS will be position-aware-per-segment. Slice 1 does NOT emit
// EntityVisibility — entity-knowledge accumulation is a future slice.
//
// The mover parameter is reserved for future slices (entity-visibility
// accumulation will use it to record "X became visible to viewer").
func ProjectMove(
	_ core.EntityID, // mover — reserved for future-slice entity-visibility
	path []core.Hex,
	viewer *View,
) (moveSlice *events.MovePlayerSlice, revealSlice *events.HexRevealedSlice, visible core.HexSet) {
	if viewer == nil || len(path) == 0 {
		return nil, nil, make(core.HexSet)
	}
	visible = VisibleHexesAt(viewer.Position, viewer.SightRange)

	var seen []core.Hex
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
	return moveSlice, nil, visible
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
	_ core.EntityID, // door — reserved for future-slice wall logic
	doorPos core.Hex,
	_ core.EntityID, // openedBy — reserved for future-slice entity-visibility
	viewer *View,
) (doorSlice *events.DoorOpenedPlayerSlice, revealSlice *events.HexRevealedSlice) {
	if viewer == nil {
		return nil, nil
	}
	visible := VisibleHexesAt(viewer.Position, viewer.SightRange)
	if !visible.Has(doorPos) {
		return nil, nil
	}

	doorSlice = &events.DoorOpenedPlayerSlice{Visible: true}

	newHexes := make(core.HexSet)
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
